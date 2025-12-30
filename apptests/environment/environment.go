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

	_ "embed"

	"github.com/drone/envsubst"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	runclient "github.com/fluxcd/pkg/runtime/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	typedclient "github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/docker"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	"github.com/mesosphere/kommander-applications/apptests/kustomize"
	"github.com/mesosphere/kommander-applications/apptests/net"
)

const (
	kommanderFluxNamespace = "kommander-flux"
	kommanderNamespace     = "kommander"
	pollInterval           = 2 * time.Second

	// ManagementClusterName is the default name for the management cluster in multi-cluster setups.
	ManagementClusterName = "management"
	// WorkloadClusterName is the default name for the workload cluster in multi-cluster setups.
	WorkloadClusterName = "workload"
)

// Env holds the configuration and state for application specific testings.
// It contains the Kubernetes client, and the kind cluster.
// In multi-cluster mode, the K8sClient, Client, Cluster are used for the management cluster.
type Env struct {
	// K8sClient is a reference to the Kubernetes client for the management cluster.
	K8sClient *typedclient.Client
	Client    genericClient.Client
	// Cluster is a dedicated instance of a kind cluster (management cluster)
	Cluster *kind.Cluster
	Network *docker.NetworkResource

	// used in multi-cluster test
	WorkloadK8sClient *typedclient.Client
	WorkloadClient    genericClient.Client
	WorkloadCluster   *kind.Cluster

	subnet      *net.Subnet
	networkName string
	subnetCIDR  string
}

//go:embed calico.yaml
var calicoYamlFile []byte

//go:embed crds/cert-manager.crds.yaml
var certManagerCRDsYamlFile []byte

// Provision creates and configures the environment for application specific testings.
// It calls the provisionEnv function and assigns the returned references to the Environment fields.
// It returns an error if any of the steps fails.
func (e *Env) Provision(ctx context.Context) error {
	cluster, k8sClient, err := provisionEnv(ctx)
	if err != nil {
		return err
	}

	e.K8sClient = k8sClient
	e.SetCluster(cluster)
	c, err := genericClient.New(k8sClient.Config(), genericClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return err
	}
	e.Client = c

	err = e.ApplyYAMLFileRaw(ctx, calicoYamlFile, nil)
	if err != nil {
		return err
	}

	err = e.ApplyYAMLFileRaw(ctx, certManagerCRDsYamlFile, nil)
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

// MultiClusterOption is a functional option for configuring multi-cluster provisioning.
type MultiClusterOption func(*Env)

// ProvisionMultiCluster creates and configures both management and workload clusters on a shared Docker network.
// The existing Env fields (K8sClient, Client, Cluster) are used for the management cluster.
// The workload cluster fields (WorkloadK8sClient, WorkloadClient, WorkloadCluster) are populated for the workload cluster.
func (e *Env) ProvisionMultiCluster(ctx context.Context, opts ...MultiClusterOption) error {
	// Apply default network name
	e.networkName = kind.GetDockerNetworkName()

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	if err := e.setupMultiClusterNetwork(ctx); err != nil {
		return fmt.Errorf("failed to setup network: %w", err)
	}

	if err := e.provisionManagementCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision management cluster: %w", err)
	}

	if err := e.provisionWorkloadCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision workload cluster: %w", err)
	}

	return nil
}

// DestroyMultiCluster cleans up both clusters and the shared network.
func (e *Env) DestroyMultiCluster(ctx context.Context) error {
	var errs []error

	// Delete the workload cluster first
	if e.WorkloadCluster != nil {
		if err := e.WorkloadCluster.Delete(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete workload cluster: %w", err))
		}
	}

	// Delete the management cluster
	if e.Cluster != nil {
		if err := e.Cluster.Delete(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete management cluster: %w", err))
		}
	}

	// Delete the Docker network
	if e.networkName != "" {
		if err := kind.EnsureNetworkIsDeleted(ctx, e.networkName); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete network: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errs)
	}

	return nil
}

// WorkloadKubeconfigPath returns the kubeconfig path for the workload cluster.
func (e *Env) WorkloadKubeconfigPath() string {
	if e.WorkloadCluster == nil {
		return ""
	}
	return e.WorkloadCluster.KubeconfigFilePath()
}

// WorkloadKubeconfigForPeers returns a kubeconfig that can be used by containers
// on the same Docker network to access the workload cluster.
func (e *Env) WorkloadKubeconfigForPeers() (string, error) {
	if e.WorkloadCluster == nil {
		return "", fmt.Errorf("workload cluster is not provisioned")
	}
	return e.WorkloadCluster.KubeconfigForPeers()
}

// KubeconfigForPeers returns a kubeconfig that can be used by containers
// on the same Docker network to access the management cluster.
func (e *Env) KubeconfigForPeers() (string, error) {
	if e.Cluster == nil {
		return "", fmt.Errorf("management cluster is not provisioned")
	}
	return e.Cluster.KubeconfigForPeers()
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

	kubeconfigPath := e.Cluster.KubeconfigFilePath()
	kubeconfigArgs := genericclioptions.NewConfigFlags(true)
	kubeconfigArgs.KubeConfig = &kubeconfigPath

	components := []string{"source-controller", "kustomize-controller", "helm-controller"}
	err := flux.Install(ctx, flux.Options{
		KubeconfigArgs:    kubeconfigArgs,
		KubeclientOptions: new(runclient.Options),
		Namespace:         kommanderFluxNamespace,
		Components:        components,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = waitForFluxDeploymentsReady(ctx, e.K8sClient)
	if err != nil {
		return err
	}

	return nil
}

// InstallLatestFluxOnWorkload installs the latest version flux components on the workload cluster.
// This method is used in multi-cluster setups to install Flux on the workload cluster.
func (e *Env) InstallLatestFluxOnWorkload(ctx context.Context) error {
	if e.WorkloadK8sClient == nil || e.WorkloadCluster == nil {
		return fmt.Errorf("workload cluster is not provisioned")
	}

	// creating the necessary namespaces
	for _, ns := range []string{kommanderFluxNamespace} {
		namespaces := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		if _, err := e.WorkloadK8sClient.Clientset().
			CoreV1().
			Namespaces().
			Create(ctx, &namespaces, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	kubeconfigPath := e.WorkloadCluster.KubeconfigFilePath()
	kubeconfigArgs := genericclioptions.NewConfigFlags(true)
	kubeconfigArgs.KubeConfig = &kubeconfigPath

	components := []string{"source-controller", "kustomize-controller", "helm-controller"}
	err := flux.Install(ctx, flux.Options{
		KubeconfigArgs:    kubeconfigArgs,
		KubeclientOptions: new(runclient.Options),
		Namespace:         kommanderFluxNamespace,
		Components:        components,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	err = waitForFluxDeploymentsReady(ctx, e.WorkloadK8sClient)
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

	// areAllDeploymentsReady is a condition function that lists deployments and checks if all are ready
	areAllDeploymentsReady := func(ctx context.Context) (done bool, err error) {
		deployments, err := typedClient.Clientset().AppsV1().
			Deployments(kommanderFluxNamespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: selector.String(),
			})
		if err != nil {
			return false, err
		}

		// If no deployments found yet, keep waiting
		if len(deployments.Items) == 0 {
			return false, nil
		}

		// Check if all deployments are ready
		for _, deployment := range deployments.Items {
			if deployment.Generation > deployment.Status.ObservedGeneration {
				return false, nil
			}
			if deployment.Status.ReadyReplicas != deployment.Status.Replicas {
				return false, nil
			}
		}

		return true, nil
	}

	return wait.PollUntilContextCancel(ctx, pollInterval, false, areAllDeploymentsReady)
}

func (e *Env) SetCluster(cluster *kind.Cluster) {
	e.Cluster = cluster
}

func (e *Env) SetClient(client genericClient.Client) {
	e.Client = client
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

		err = e.Client.Patch(ctx, &obj, genericClient.Apply, genericClient.ForceOwnership, genericClient.FieldOwner("k-cli"))
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

	// Loop through the files in the specified directory and apply the YAML files to the cluster
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		// Skip the kustomize.yaml files if they exist
		if info.Name() == "kustomization.yaml" {
			return nil
		}

		err = applyYAMLFile(ctx, e.Client, path, substitutions, true)
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

	// Loop through the files in the specified directory and apply the YAML files to the cluster
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		err = applyYAMLFile(ctx, e.Client, path, nil, false)
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
func applyYAMLFile(ctx context.Context, client genericClient.Client, path string, substitutions map[string]string, applySubstitutions bool) error {
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

		err = client.Patch(ctx, &obj, genericClient.Apply, genericClient.ForceOwnership, genericClient.FieldOwner("k-cli"))
		if err != nil {
			return fmt.Errorf("could not patch the resources for path: %s :%w", path, err)
		}
	}

	return nil
}

// ApplyYAMLFileRaw applies the YAML file provided to the primary/management cluster.
func (e *Env) ApplyYAMLFileRaw(ctx context.Context, file []byte, substitutions map[string]string) error {
	return applyYAMLFileRawToClient(ctx, e.Client, file, substitutions)
}

// applyYAMLFileRawToClient applies the YAML file to the specified client.
func applyYAMLFileRawToClient(ctx context.Context, client genericClient.Client, file []byte, substitutions map[string]string) error {
	var err error
	log.SetLogger(klog.NewKlogr())

	// Substitute the environment variables in the YAML file
	var yml string
	if substitutions != nil {
		yml, err = envsubst.Eval(string(file), func(s string) string {
			return substitutions[s]
		})
		if err != nil {
			return err
		}
	} else {
		yml = string(file)
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
			return fmt.Errorf("could not decode yaml file : %w", err)
		}

		err = client.Patch(ctx, &obj, genericClient.Apply, genericClient.ForceOwnership, genericClient.FieldOwner("k-cli"))
		if err != nil {
			return fmt.Errorf("could not patch the resources for path :%w", err)
		}
	}

	return nil
}
