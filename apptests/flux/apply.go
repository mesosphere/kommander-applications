package flux

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/api/konfig"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen/kustomization"
	helmv2b2 "github.com/fluxcd/helm-controller/api/v2beta2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	runclient "github.com/fluxcd/pkg/runtime/client"
	"github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautils "github.com/fluxcd/pkg/ssa/utils"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
	gatekeeperapi "github.com/open-policy-agent/frameworks/constraint/pkg/apis"
	traefikv1a1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
)

// Apply is the equivalent of 'kubectl apply --server-side -f'.
// If the given manifest is a kustomization.yaml, then apply performs the equivalent of 'kubectl apply --server-side -k'.
func Apply(ctx context.Context, rcg genericclioptions.RESTClientGetter, opts *runclient.Options, root, manifestPath string) (string, error) {
	objs, err := readObjects(root, manifestPath)
	if err != nil {
		return "", err
	}

	if len(objs) == 0 {
		return "", fmt.Errorf("no Kubernetes objects found at: %s", manifestPath)
	}

	if err := normalize.UnstructuredList(objs); err != nil {
		return "", err
	}

	changeSet := ssa.NewChangeSet()

	// contains only CRDs and Namespaces
	var stageOne []*unstructured.Unstructured

	// contains all objects except for CRDs and Namespaces
	var stageTwo []*unstructured.Unstructured

	for _, u := range objs {
		if ssautils.IsClusterDefinition(u) {
			stageOne = append(stageOne, u)
		} else {
			stageTwo = append(stageTwo, u)
		}
	}

	if len(stageOne) > 0 {
		cs, err := applySet(ctx, rcg, opts, stageOne)
		if err != nil {
			return "", err
		}
		changeSet.Append(cs.Entries)
	}

	if err := waitForSet(rcg, opts, changeSet); err != nil {
		return "", err
	}

	if len(stageTwo) > 0 {
		cs, err := applySet(ctx, rcg, opts, stageTwo)
		if err != nil {
			return "", err
		}
		changeSet.Append(cs.Entries)
	}

	return changeSet.String(), nil
}

func readObjects(root, manifestPath string) ([]*unstructured.Unstructured, error) {
	fi, err := os.Lstat(manifestPath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("expected %q to be a file", manifestPath)
	}

	if isRecognizedKustomizationFile(manifestPath) {
		resources, err := kustomization.BuildWithRoot(root, filepath.Dir(manifestPath))
		if err != nil {
			return nil, err
		}
		return ssautils.ReadObjects(bytes.NewReader(resources))
	}

	ms, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer ms.Close()

	return ssautils.ReadObjects(bufio.NewReader(ms))
}

func newManager(rcg genericclioptions.RESTClientGetter, opts *runclient.Options) (*ssa.ResourceManager, error) {
	log.SetLogger(klog.NewKlogr())
	cfg, err := KubeConfig(rcg, opts)
	if err != nil {
		return nil, err
	}
	restMapper, err := rcg.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	kubeClient, err := client.New(cfg, client.Options{Mapper: restMapper, Scheme: NewScheme()})
	if err != nil {
		return nil, err
	}
	kubePoller := polling.NewStatusPoller(kubeClient, restMapper, polling.Options{})

	return ssa.NewResourceManager(kubeClient, kubePoller, ssa.Owner{
		Field: "flux",
		Group: "fluxcd.io",
	}), nil
}

// Create the Scheme, methods for serializing and deserializing API objects
// which can be shared by tests.
func NewScheme() *apiruntime.Scheme {
	scheme := apiruntime.NewScheme()
	_ = apiextensionsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = sourcev1b2.AddToScheme(scheme)
	_ = sourcev1.AddToScheme(scheme)
	_ = kustomizev1.AddToScheme(scheme)
	_ = helmv2b2.AddToScheme(scheme)
	_ = traefikv1a1.AddToScheme(scheme)
	_ = gatekeeperapi.AddToScheme(scheme)
	return scheme
}

func KubeConfig(rcg genericclioptions.RESTClientGetter, opts *runclient.Options) (*rest.Config, error) {
	cfg, err := rcg.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes configuration load failed: %w", err)
	}

	// avoid throttling request when some Flux CRDs are not registered
	cfg.QPS = opts.QPS
	cfg.Burst = opts.Burst

	return cfg, nil
}

func applySet(ctx context.Context, rcg genericclioptions.RESTClientGetter, opts *runclient.Options, objects []*unstructured.Unstructured) (*ssa.ChangeSet, error) {
	man, err := newManager(rcg, opts)
	if err != nil {
		return nil, err
	}

	return man.ApplyAll(ctx, objects, ssa.DefaultApplyOptions())
}

func waitForSet(rcg genericclioptions.RESTClientGetter, opts *runclient.Options, changeSet *ssa.ChangeSet) error {
	man, err := newManager(rcg, opts)
	if err != nil {
		return err
	}
	return man.WaitForSet(changeSet.ToObjMetadataSet(), ssa.WaitOptions{Interval: 2 * time.Second, Timeout: time.Minute})
}

func isRecognizedKustomizationFile(path string) bool {
	base := filepath.Base(path)
	for _, v := range konfig.RecognizedKustomizationFileNames() {
		if base == v {
			return true
		}
	}
	return false
}
