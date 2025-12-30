// Package environment provides functions to manage and configure environment for application specific testings.
package environment

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"

	typedclient "github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/docker"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	"github.com/mesosphere/kommander-applications/apptests/net"
)

const (
	ManagementClusterName = "management"
	WorkloadClusterName   = "workload"
)

// MultiClusterEnv holds the configuration and state for multi-cluster testing environments.
// It manages two Kind clusters (management and workload) that can communicate via a shared Docker network.
type MultiClusterEnv struct {
	ManagementEnv *Env
	WorkloadEnv   *Env
	Network       *docker.NetworkResource
	Subnet        *net.Subnet
	networkName   string
	subnetCIDR    string
}

// MultiClusterOption is a functional option for configuring MultiClusterEnv.
type MultiClusterOption func(*MultiClusterEnv)

// WithSubnet sets a custom subnet for the multi-cluster environment.
func WithSubnet(subnet string) MultiClusterOption {
	return func(m *MultiClusterEnv) {
		m.subnetCIDR = subnet
	}
}

// WithNetworkName sets a custom network name for the multi-cluster environment.
func WithNetworkName(name string) MultiClusterOption {
	return func(m *MultiClusterEnv) {
		m.networkName = name
	}
}

// NewMultiClusterEnv creates a new multi-cluster environment with the given options.
// By default, it uses an empty subnet which allows reusing an existing network without subnet validation.
// Use WithSubnet(DefaultMultiClusterSubnet) if you need a specific IPv4 subnet.
func NewMultiClusterEnv(opts ...MultiClusterOption) *MultiClusterEnv {
	m := &MultiClusterEnv{
		ManagementEnv: &Env{},
		WorkloadEnv:   &Env{},
		subnetCIDR:    "", // Empty to allow reusing existing network without subnet validation
		networkName:   kind.GetDockerNetworkName(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Provision creates and configures both clusters on a shared Docker network.
// It sets up the network, creates both clusters, and configures networking components (MetalLB, Calico).
func (m *MultiClusterEnv) Provision(ctx context.Context) error {
	if err := m.setupNetwork(ctx); err != nil {
		return fmt.Errorf("failed to setup network: %w", err)
	}

	if err := m.provisionManagementCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision management cluster: %w", err)
	}

	if err := m.provisionWorkloadCluster(ctx); err != nil {
		return fmt.Errorf("failed to provision workload cluster: %w", err)
	}

	return nil
}

// setupNetwork ensures the Docker network exists and configures the subnet.
func (m *MultiClusterEnv) setupNetwork(ctx context.Context) error {
	network, err := kind.EnsureDockerNetworkExist(ctx, m.subnetCIDR, false)
	if err != nil {
		return err
	}

	m.Network = network
	m.ManagementEnv.Network = network
	m.WorkloadEnv.Network = network

	subnet, err := network.Subnet()
	if err != nil {
		return fmt.Errorf("failed to get subnet from network: %w", err)
	}
	m.Subnet = subnet

	return nil
}

// provisionManagementCluster creates and configures the management cluster.
func (m *MultiClusterEnv) provisionManagementCluster(ctx context.Context) error {
	// Create the cluster in the shared network
	cluster, err := kind.CreateClusterInNetwork(ctx, ManagementClusterName, m.networkName)
	if err != nil {
		return fmt.Errorf("failed to create management cluster: %w", err)
	}
	m.ManagementEnv.Cluster = cluster

	// Setup Kubernetes client
	k8sClient, err := typedclient.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return fmt.Errorf("failed to create k8s client for management cluster: %w", err)
	}
	m.ManagementEnv.K8sClient = k8sClient

	// Setup generic client
	genericCl, err := genericClient.New(k8sClient.Config(), genericClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("failed to create generic client for management cluster: %w", err)
	}
	m.ManagementEnv.Client = genericCl

	if err := m.createNamespace(ctx, m.ManagementEnv.K8sClient, kommanderNamespace); err != nil {
		return fmt.Errorf("failed to create kommander namespace on management cluster: %w", err)
	}

	if err := m.ManagementEnv.ApplyYAMLFileRaw(ctx, calicoYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply calico on management cluster: %w", err)
	}

	// Apply cert-manager CRDs
	if err := m.ManagementEnv.ApplyYAMLFileRaw(ctx, certManagerCRDsYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs on management cluster: %w", err)
	}

	// Install MetalLB - it will allocate an IP range from the subnet
	_ = InstallMetallb(ctx, cluster.KubeconfigFilePath(), m.Subnet)

	return nil
}

// provisionWorkloadCluster creates and configures the workload cluster.
func (m *MultiClusterEnv) provisionWorkloadCluster(ctx context.Context) error {
	cluster, err := kind.CreateClusterInNetwork(ctx, WorkloadClusterName, m.networkName)
	if err != nil {
		return fmt.Errorf("failed to create workload cluster: %w", err)
	}
	m.WorkloadEnv.Cluster = cluster

	k8sClient, err := typedclient.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return fmt.Errorf("failed to create k8s client for workload cluster: %w", err)
	}
	m.WorkloadEnv.K8sClient = k8sClient

	genericCl, err := genericClient.New(k8sClient.Config(), genericClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("failed to create generic client for workload cluster: %w", err)
	}
	m.WorkloadEnv.Client = genericCl

	if err := m.createNamespace(ctx, m.WorkloadEnv.K8sClient, kommanderNamespace); err != nil {
		return fmt.Errorf("failed to create kommander namespace on workload cluster: %w", err)
	}

	if err := m.WorkloadEnv.ApplyYAMLFileRaw(ctx, calicoYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply calico on workload cluster: %w", err)
	}

	// Apply cert-manager CRDs
	if err := m.WorkloadEnv.ApplyYAMLFileRaw(ctx, certManagerCRDsYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs on workload cluster: %w", err)
	}

	// Install MetalLB - it will allocate an IP range from the subnet
	_ = InstallMetallb(ctx, cluster.KubeconfigFilePath(), m.Subnet)

	return nil
}

// createNamespace creates a namespace in the cluster if it doesn't exist.
func (m *MultiClusterEnv) createNamespace(ctx context.Context, client *typedclient.Client, name string) error {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := client.Clientset().CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
	return err
}

// Destroy cleans up both clusters and the shared network.
func (m *MultiClusterEnv) Destroy(ctx context.Context) error {
	var errs []error

	// Delete the workload cluster first
	if m.WorkloadEnv != nil && m.WorkloadEnv.Cluster != nil {
		if err := m.WorkloadEnv.Cluster.Delete(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete workload cluster: %w", err))
		}
	}

	// Delete the management cluster
	if m.ManagementEnv != nil && m.ManagementEnv.Cluster != nil {
		if err := m.ManagementEnv.Cluster.Delete(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete management cluster: %w", err))
		}
	}

	// Delete the Docker network
	if err := kind.EnsureNetworkIsDeleted(ctx, m.networkName); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete network: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errs)
	}

	return nil
}

// ManagementCluster returns the management cluster.
func (m *MultiClusterEnv) ManagementCluster() *kind.Cluster {
	return m.ManagementEnv.Cluster
}

// WorkloadCluster returns the workload cluster.
func (m *MultiClusterEnv) WorkloadCluster() *kind.Cluster {
	return m.WorkloadEnv.Cluster
}

// ManagementClient returns the Kubernetes client for the management cluster.
func (m *MultiClusterEnv) ManagementClient() genericClient.Client {
	return m.ManagementEnv.Client
}

// WorkloadClient returns the Kubernetes client for the workload cluster.
func (m *MultiClusterEnv) WorkloadClient() genericClient.Client {
	return m.WorkloadEnv.Client
}

// ManagementKubeconfigPath returns the kubeconfig path for the management cluster.
func (m *MultiClusterEnv) ManagementKubeconfigPath() string {
	if m.ManagementEnv.Cluster == nil {
		return ""
	}
	return m.ManagementEnv.Cluster.KubeconfigFilePath()
}

// WorkloadKubeconfigPath returns the kubeconfig path for the workload cluster.
func (m *MultiClusterEnv) WorkloadKubeconfigPath() string {
	if m.WorkloadEnv.Cluster == nil {
		return ""
	}
	return m.WorkloadEnv.Cluster.KubeconfigFilePath()
}

// ManagementKubeconfigForPeers returns a kubeconfig that can be used by containers
// on the same Docker network to access the management cluster.
func (m *MultiClusterEnv) ManagementKubeconfigForPeers() (string, error) {
	if m.ManagementEnv.Cluster == nil {
		return "", fmt.Errorf("management cluster is not provisioned")
	}
	return m.ManagementEnv.Cluster.KubeconfigForPeers()
}

// WorkloadKubeconfigForPeers returns a kubeconfig that can be used by containers
// on the same Docker network to access the workload cluster.
func (m *MultiClusterEnv) WorkloadKubeconfigForPeers() (string, error) {
	if m.WorkloadEnv.Cluster == nil {
		return "", fmt.Errorf("workload cluster is not provisioned")
	}
	return m.WorkloadEnv.Cluster.KubeconfigForPeers()
}
