// Package environment provides functions to manage, and configure environment for application specific testings.
package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	runclient "github.com/fluxcd/pkg/runtime/client"
	typedclient "github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	"github.com/mesosphere/kommander-applications/apptests/kustomize"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	kommanderFluxNamespace = "kommander-flux"
	kommanderNamespace     = "kommander"
	pollInterval           = 2 * time.Second
)

// Env holds the configuration and state for application specific testings.
// It contains the Kubernetes client, and the kind cluster.
type Env struct {
	// K8sClient is a reference to the Kubernetes client
	// This client is used to interact with the cluster built during the execution of the application specific testing.
	K8sClient *typedclient.Client

	// Cluster is a dedicated instance of a kind cluster created for running an application specific test.
	Cluster *kind.Cluster
}

// Provision creates and configures the environment for application specific testings.
// It calls the provisionEnv function and assigns the returned references to the Environment fields.
// It returns an error if any of the steps fails.
func (e *Env) Provision(ctx context.Context) error {
	var err error

	kustomizePath, err := absolutePathToBase()
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
func provisionEnv(ctx context.Context) (*kind.Cluster, *typedclient.Client, error) {
	cluster, err := kind.CreateCluster(ctx, "")
	if err != nil {
		return nil, nil, err
	}

	client, err := typedclient.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return nil, nil, err
	}

	// creating the necessary namespaces
	for _, ns := range []string{kommanderNamespace, kommanderFluxNamespace} {
		namespaces := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		if _, err = client.Clientset().
			CoreV1().
			Namespaces().
			Create(ctx, &namespaces, metav1.CreateOptions{}); err != nil {
			return nil, nil, err
		}
	}

	components := []string{"source-controller", "kustomize-controller", "helm-controller"}
	err = flux.Install(ctx, flux.Options{
		KubeconfigArgs:    genericclioptions.NewConfigFlags(true),
		KubeclientOptions: new(runclient.Options),
		Namespace:         kommanderFluxNamespace,
		Components:        components,
	})

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = waitForFluxDeploymentsReady(ctx, client)
	if err != nil {
		return nil, nil, err
	}

	return cluster, client, err
}

// waitForFluxDeploymentsReady discovers all flux deployments in the kommander-flux namespace and waits until they get ready
// it returns an error if the context is cancelled or expired, the deployments are missing, or not ready
func waitForFluxDeploymentsReady(ctx context.Context, typedClient *typedclient.Client) error {
	selector := labels.SelectorFromSet(map[string]string{
		manifestgen.PartOfLabelKey:   manifestgen.PartOfLabelValue,
		manifestgen.InstanceLabelKey: kommanderFluxNamespace,
	})

	deployments, err := typedClient.Clientset().AppsV1().
		Deployments(kommanderFluxNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
	if err != nil {
		return err
	}
	if len(deployments.Items) == 0 {
		return fmt.Errorf(
			"no flux conrollers found in the namespace: %s with the label selector %s",
			kommanderFluxNamespace, selector.String())
	}

	// isDeploymentReady is a condition function that checks a single deployment readiness
	isDeploymentReady := func(ctx context.Context, deployment appsv1.Deployment) wait.ConditionWithContextFunc {
		return func(ctx context.Context) (done bool, err error) {
			deploymentObj, err := typedClient.Clientset().AppsV1().
				Deployments(deployment.Namespace).
				Get(ctx, deployment.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if deploymentObj.Generation > deploymentObj.Status.ObservedGeneration {
				return false, nil
			}

			return deploymentObj.Status.ReadyReplicas == deploymentObj.Status.Replicas, nil
		}
	}

	for _, deployment := range deployments.Items {
		err = wait.PollUntilContextCancel(ctx, pollInterval, false, isDeploymentReady(ctx, deployment))
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Env) SetCluster(cluster *kind.Cluster) {
	e.Cluster = cluster
}

func (e *Env) SetK8sClient(k8sClient *typedclient.Client) {
	e.K8sClient = k8sClient
}

// ApplyKustomizations applies the kustomizations located in the given path.
func (e *Env) ApplyKustomizations(ctx context.Context, path string, substitutions map[string]string) error {
	log.SetLogger(klog.NewKlogr())

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
	genericClient, err := genericCLient.New(e.K8sClient.Config(), genericCLient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("could not create the generic client for path: %s :%w", path, err)
	}

	for {
		var obj unstructured.Unstructured
		err = dec.Decode(&obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not decode kustomization for path: %s :%w", path, err)
		}

		err = genericClient.Patch(ctx, &obj, genericCLient.Apply, genericCLient.ForceOwnership, genericCLient.FieldOwner("k-cli"))
		if err != nil {
			return fmt.Errorf("could not patch the kustomization resources for path: %s :%w", path, err)
		}
	}

	return nil
}

// absolutePathToBase returns the absolute path to common/base directory from the given working directory.
func absolutePathToBase() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// determining the execution path.
	var base string
	_, err = os.Stat(filepath.Join(wd, "common", "base"))
	if os.IsNotExist(err) {
		base = "../.."
	} else {
		base = ""
	}

	return filepath.Join(wd, base, "common", "base"), nil
}
