// Package environment provides functions to manage and configure environment for application specific testings.
package environment

import (
	"context"
	"fmt"

	typedclient "github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// setupMultiClusterNetwork ensures the Docker network exists and configures the subnet.
func (e *Env) setupMultiClusterNetwork(ctx context.Context) error {
	network, err := kind.EnsureDockerNetworkExist(ctx, e.subnetCIDR, false)
	if err != nil {
		return err
	}

	e.Network = network

	subnet, err := network.Subnet()
	if err != nil {
		return fmt.Errorf("failed to get subnet from network: %w", err)
	}
	e.subnet = subnet

	return nil
}

// provisionManagementCluster creates and configures the management cluster.
// It populates the primary Env fields: K8sClient, Client, Cluster.
func (e *Env) provisionManagementCluster(ctx context.Context) error {
	// Create the cluster in the shared network
	cluster, err := kind.CreateClusterInNetwork(ctx, ManagementClusterName, e.networkName)
	if err != nil {
		return fmt.Errorf("failed to create management cluster: %w", err)
	}
	e.Cluster = cluster

	// Setup Kubernetes client
	k8sClient, err := typedclient.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return fmt.Errorf("failed to create k8s client for management cluster: %w", err)
	}
	e.K8sClient = k8sClient

	// Setup generic client
	genericCl, err := genericClient.New(k8sClient.Config(), genericClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("failed to create generic client for management cluster: %w", err)
	}
	e.Client = genericCl

	if err := e.createNamespaceWithClient(ctx, e.K8sClient, kommanderNamespace); err != nil {
		return fmt.Errorf("failed to create kommander namespace on management cluster: %w", err)
	}

	if err := e.ApplyYAMLFileRaw(ctx, calicoYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply calico on management cluster: %w", err)
	}

	// Apply cert-manager CRDs
	if err := e.ApplyYAMLFileRaw(ctx, certManagerCRDsYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs on management cluster: %w", err)
	}

	// Install MetalLB - it will allocate an IP range from the subnet
	_ = InstallMetallb(ctx, cluster.KubeconfigFilePath(), e.subnet)

	return nil
}

// provisionWorkloadCluster creates and configures the workload cluster.
// It populates the workload Env fields: WorkloadK8sClient, WorkloadClient, WorkloadCluster.
func (e *Env) provisionWorkloadCluster(ctx context.Context) error {
	cluster, err := kind.CreateClusterInNetwork(ctx, WorkloadClusterName, e.networkName)
	if err != nil {
		return fmt.Errorf("failed to create workload cluster: %w", err)
	}
	e.WorkloadCluster = cluster

	k8sClient, err := typedclient.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return fmt.Errorf("failed to create k8s client for workload cluster: %w", err)
	}
	e.WorkloadK8sClient = k8sClient

	genericCl, err := genericClient.New(k8sClient.Config(), genericClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("failed to create generic client for workload cluster: %w", err)
	}
	e.WorkloadClient = genericCl

	if err := e.createNamespaceWithClient(ctx, e.WorkloadK8sClient, kommanderNamespace); err != nil {
		return fmt.Errorf("failed to create kommander namespace on workload cluster: %w", err)
	}

	// Apply calico using workload client
	if err := e.applyYAMLFileRawWithClient(ctx, e.WorkloadClient, calicoYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply calico on workload cluster: %w", err)
	}

	// Apply cert-manager CRDs using workload client
	if err := e.applyYAMLFileRawWithClient(ctx, e.WorkloadClient, certManagerCRDsYamlFile, nil); err != nil {
		return fmt.Errorf("failed to apply cert-manager CRDs on workload cluster: %w", err)
	}

	// Install MetalLB - it will allocate an IP range from the subnet
	_ = InstallMetallb(ctx, cluster.KubeconfigFilePath(), e.subnet)

	return nil
}

// createNamespaceWithClient creates a namespace in the cluster using the specified client.
func (e *Env) createNamespaceWithClient(ctx context.Context, client *typedclient.Client, name string) error {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := client.Clientset().CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
	return err
}

// applyYAMLFileRawWithClient applies the YAML file using a specific client.
func (e *Env) applyYAMLFileRawWithClient(ctx context.Context, client genericClient.Client, file []byte, substitutions map[string]string) error {
	return applyYAMLFileRawToClient(ctx, client, file, substitutions, []string{})
}
