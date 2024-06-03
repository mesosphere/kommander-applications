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

	"github.com/drone/envsubst"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	runclient "github.com/fluxcd/pkg/runtime/client"
	typedclient "github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/docker"
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
	Network *docker.NetworkResource
}

// Provision creates and configures the environment for application specific testings.
// It calls the provisionEnv function and assigns the returned references to the Environment fields.
// It returns an error if any of the steps fails.
func (e *Env) Provision(ctx context.Context) error {
	cluster, k8sClient, err := provisionEnv(ctx)
	if err != nil {
		return err
	}

	e.SetK8sClient(k8sClient)
	e.SetCluster(cluster)
	// install calico CNI
	err = e.ApplyYAML(ctx, "../environment/calico.yaml", nil)
	if err != nil {
		return err
	}

	subnet, err := e.Network.Subnet()
	if err != nil {
		return err
	}
	_ = InstallMetallb(ctx, e.Cluster.KubeconfigFilePath(), subnet)

	return nil
}

// Destroy deletes the provisioned cluster if it exists.
func (e *Env) Destroy(ctx context.Context) error {
	if e.Cluster != nil {
		return e.Cluster.Delete(ctx)
	}

	return nil
}

// provisionEnv creates a kind cluster, a Kubernetes client, and installs metallb and calico components on the cluster.
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
	for _, ns := range []string{kommanderNamespace} {
		namespaces := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		if _, err = client.Clientset().
			CoreV1().
			Namespaces().
			Create(ctx, &namespaces, metav1.CreateOptions{}); err != nil {
			return nil, nil, err
		}
	}

	return cluster, client, err
}

// InstallBaseFlux installs the latest version flux components on the cluster. Not the same as installing kommander-flux
// from the catalog.
func (e *Env) InstallLatestFlux(ctx context.Context) error {
	// creating the necessary namespaces
	for _, ns := range []string{kommanderFluxNamespace} {
		namespaces := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		if _, err := e.K8sClient.Clientset().
			CoreV1().
			Namespaces().
			Create(ctx, &namespaces, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	components := []string{"source-controller", "kustomize-controller", "helm-controller"}
	err := flux.Install(ctx, flux.Options{
		KubeconfigArgs:    genericclioptions.NewConfigFlags(true),
		KubeclientOptions: new(runclient.Options),
		Namespace:         kommanderFluxNamespace,
		Components:        components,
	})

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = waitForFluxDeploymentsReady(ctx, e.K8sClient)
	if err != nil {
		return err
	}

	return nil
}

// RunScriptAllNode runs a script on all nodes in the cluster using Docker
func (e *Env) RunScriptOnAllNode(ctx context.Context, script string) error {
	nodes, err := e.Cluster.ListNodeNames(ctx)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		err = e.Cluster.RunScript(ctx, node, script)
		if err != nil {
			return err
		}
	}

	return nil
}

// ApplyKommanderBaseKustomizations applies the base Kustomizations from the common directory in the catalog. This
// creates the HelmRepositories and installs the dkp priority classes.
func (e *Env) ApplyKommanderBaseKustomizations(ctx context.Context) error {
	kustomizePath, err := absolutePathToBase()
	if err != nil {
		return err
	}

	// apply base Kustomizations
	err = e.ApplyKustomizations(ctx, kustomizePath, nil)
	if err != nil {
		return err
	}

	return nil
}

// ApplyKommanderPriorityClasses applies the priority classes only from the base resources.
func (e *Env) ApplyKommanderPriorityClasses(ctx context.Context) error {
	kustomizePath, err := absolutePathToBase()
	if err != nil {
		return err
	}

	// get priority classes path
	kustomizePath = filepath.Join(kustomizePath, "../priority-classes")

	// apply priority classes
	err = e.ApplyKustomizations(ctx, kustomizePath, nil)
	if err != nil {
		return err
	}

	return nil
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

// ApplyKustomizations applies the kustomizations located in the given path and does variable substitution.
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

// ApplyYAML applies the YAML manifests located in the given directory and does variable substitution.
func (e *Env) ApplyYAML(ctx context.Context, path string, substitutions map[string]string) error {
	log.SetLogger(klog.NewKlogr())

	if path == "" {
		return fmt.Errorf("requirement argument: path is not specified")
	}

	genericClient, err := genericCLient.New(e.K8sClient.Config(), genericCLient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("could not create the generic client for path: %s :%w", path, err)
	}

	// Loop through the files in the specified directory and apply the YAML files to the cluster
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		err = applyYAMLFile(ctx, genericClient, path, substitutions, true)
		if err != nil {
			return fmt.Errorf("could not apply the YAML file for path: %s :%w", path, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not walk the path: %s :%w", path, err)
	}

	return nil
}

// ApplyYAMLWithoutSubstitutions applies the YAML manifests located in the given directory as is and does not do variable substitution.
func (e *Env) ApplyYAMLWithoutSubstitutions(ctx context.Context, path string) error {
	log.SetLogger(klog.NewKlogr())

	if path == "" {
		return fmt.Errorf("requirement argument: path is not specified")
	}

	genericClient, err := genericCLient.New(e.K8sClient.Config(), genericCLient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("could not create the generic client for path: %s :%w", path, err)
	}

	// Loop through the files in the specified directory and apply the YAML files to the cluster
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		err = applyYAMLFile(ctx, genericClient, path, nil, false)
		if err != nil {
			return fmt.Errorf("could not apply the YAML file for path: %s :%w", path, err)
		}

		return nil

	})
	if err != nil {
		return fmt.Errorf("could not walk the path: %s :%w", path, err)
	}

	return nil
}

// applyYAMLFile applies the YAML file located in the given path.
func applyYAMLFile(ctx context.Context, genericClient genericCLient.Client, path string, substitutions map[string]string, applySubstitutions bool) error {
	// Read and decode the YAML file specified in the path
	out, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read the YAML file for path: %s :%w", path, err)
	}

	// Substitute the environment variables in the YAML file
	var yml string
	if applySubstitutions {
		yml, err = envsubst.Eval(string(out), func(s string) string {
			return substitutions[s]
		})
		if err != nil {
			return err
		}
	} else {
		yml = string(out)
	}

	// Decode the YAML file
	buf := bytes.NewBuffer([]byte(yml))
	dec := yaml.NewYAMLOrJSONDecoder(buf, 1<<20) // default buffer size is 1MB

	// Loop through the resources in the YAML file and apply them to the cluster
	for {
		var obj unstructured.Unstructured
		err = dec.Decode(&obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not decode yaml for path: %s :%w", path, err)
		}

		err = genericClient.Patch(ctx, &obj, genericCLient.Apply, genericCLient.ForceOwnership, genericCLient.FieldOwner("k-cli"))
		if err != nil {
			return fmt.Errorf("could not patch the resources for path: %s :%w", path, err)
		}
	}

	return nil
}
