// Package environment provides functions to manage, and configure environment for application specific testings.
package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	"github.com/mesosphere/kommander-applications/apptests/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	runclient "github.com/fluxcd/pkg/runtime/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kommanderFluxNamespace = "kommander-flux"
	kommanderNamespace     = "kommander"
)

// Env holds the configuration and state for application specific testings.
// It contains the applications to test, the Kubernetes client, and the kind cluster.
type Env struct {
	// K8sClient is a reference to the Kubernetes client
	// This client is used to interact with the cluster built during the execution of the application specific testing.
	K8sClient *client.Client

	// Cluster is a dedicated instance of a kind cluster created for running an application specific test.
	Cluster *kind.Cluster
}

// Provision creates and configures the environment for application specific testings.
// It calls the provisionEnv function and assigns the returned references to the Environment fields.
// It returns an error if any of the steps fails.
func (e *Env) Provision(ctx context.Context) error {
	var err error

	kustomizePath, err := AbsolutePathToBase()
	if err != nil {
		return err
	}
	// delete the cluster if any error occurs
	defer func() {
		if err != nil {
			e.Destroy(ctx)
		}
	}()

	cluster, k8sClient, err := provisionEnv(ctx)
	if err != nil {
		return err
	}

	e.SetK8sClient(k8sClient)
	e.SetCluster(cluster)

	// apply base Kustomizations
	err = e.ApplyKustomizations(ctx, kustomizePath, nil)
	if err != nil {
		return err
	}

	return nil
}

// Destroy deletes the provisioned cluster if it exists.
func (e *Env) Destroy(ctx context.Context) error {
	if e.Cluster != nil {
		return e.Cluster.Delete(ctx)
	}

	return nil
}

// provisionEnv creates a kind cluster, a Kubernetes client, and installs flux components on the cluster.
// It returns the created cluster and client references, or an error if any of the steps fails.
func provisionEnv(ctx context.Context) (*kind.Cluster, *client.Client, error) {
	cluster, err := kind.CreateCluster(ctx, "")
	if err != nil {
		return nil, nil, err
	}

	c, err := client.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return nil, nil, err
	}

	// creating the necessary namespaces
	for _, ns := range []string{kommanderNamespace, kommanderFluxNamespace} {
		namespaces := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		if _, err = c.Clientset().
			CoreV1().
			Namespaces().
			Create(ctx, &namespaces, metav1.CreateOptions{}); err != nil {
			return nil, nil, err
		}
	}

	err = flux.Install(ctx, flux.Options{
		KubeconfigArgs:    genericclioptions.NewConfigFlags(true),
		KubeclientOptions: new(runclient.Options),
		Namespace:         kommanderFluxNamespace,
		Components:        []string{"source-controller", "kustomize-controller", "helm-controller"},
	})

	return cluster, c, err
}

func (e *Env) SetCluster(cluster *kind.Cluster) {
	e.Cluster = cluster
}

func (e *Env) SetK8sClient(k8sClient *client.Client) {
	e.K8sClient = k8sClient
}

// ApplyKustomizations applies the kustomizations located in the given path.
func (e *Env) ApplyKustomizations(ctx context.Context, path string, substitutions map[string]string) error {
	if path == "" {
		return fmt.Errorf("requirement argument: path is not specified")
	}

	kustomizer := kustomize.New(path, substitutions)
	if err := kustomizer.Build(); err != nil {
		return fmt.Errorf("could not build kustomization manifest for path: %s :%w", path, err)
	}
	out, err := kustomizer.Output()
	if err != nil {
		return fmt.Errorf("could not generate YAML manifest for path: %s :%w", path, err)
	}

	buf := bytes.NewBuffer(out)
	dec := yaml.NewYAMLOrJSONDecoder(buf, 1<<20) // default buffer size is 1MB
	obj := unstructured.Unstructured{}
	if err = dec.Decode(&obj); err != nil && err != io.EOF {
		return fmt.Errorf("could not decode kustomization for path: %s :%w", path, err)
	}

	genericClient, err := genericCLient.New(e.K8sClient.Config(), genericCLient.Options{})
	if err != nil {
		return fmt.Errorf("could not create the generic client for path: %s :%w", path, err)
	}

	err = genericClient.Patch(ctx, &obj, genericCLient.Apply, genericCLient.ForceOwnership)
	if err != nil {
		return fmt.Errorf("could not patch the kustomization resources for path: %s :%w", path, err)
	}

	return nil
}

// AbsolutePathToBase returns the absolute path to common/base directory.
func AbsolutePathToBase() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, "../../common/base"), nil
}
