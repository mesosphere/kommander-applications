package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

const workspaceNSName = "workspace-1"

type openCost struct {
	// workloadNodeIP stores the Node IP of the workload cluster for Thanos to connect via NodePort
	workloadNodeIP       string
	managementServiceUrl string
}

var _ scenarios.AppScenario = (*openCost)(nil)

func NewOpenCost() *openCost {
	return &openCost{}
}

func (o *openCost) Name() string {
	return constants.OpenCost
}

// Install deploys the multi-cluster OpenCost setup including all prerequisites.
// This deploys:
// - Management cluster: thanos (pointing to mgmt and workload KPS) + kube-prometheus-stack + centralized-opencost
// - Workload cluster: kube-prometheus-stack (with NodePort) + opencost
// TODO: refactor this to call thanos and KPS structs once they are implemented
func (o *openCost) Install(ctx context.Context, env *environment.Env) error {
	if err := o.deployKPSOnWorkload(ctx, env); err != nil {
		return fmt.Errorf("failed to deploy KPS on workload cluster: %w", err)
	}

	if err := o.deployKPSOnManagement(ctx, env); err != nil {
		return fmt.Errorf("failed to deploy KPS on workload management cluster: %w", err)
	}

	nodeIP, err := o.getWorkloadNodeIP(ctx, env.WorkloadClient)
	if err != nil {
		return fmt.Errorf("failed to get workload node IP: %w", err)
	}
	o.workloadNodeIP = nodeIP
	o.managementServiceUrl = "kube-prometheus-stack-prometheus.kommander.svc.cluster.local"

	if err := o.createThanosStoresConfigMap(ctx, env); err != nil {
		return fmt.Errorf("failed to create Thanos stores ConfigMap: %w", err)
	}
	if err := o.deployThanosOnManagement(ctx, env); err != nil {
		return fmt.Errorf("failed to deploy Thanos on management cluster: %w", err)
	}

	return nil
}

func (o *openCost) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("InstallPreviousVersion is not implemented for multi-cluster OpenCost")
}

func (o *openCost) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("Upgrade is not implemented for multi-cluster OpenCost")
}

// applyKPSWorkloadOverride applies the KPS override ConfigMap on the workload cluster.
func (o *openCost) applyKPSWorkloadOverride(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	overridePath := filepath.Join(testDataPath, "opencost", "kps-workload-override.yaml")
	content, err := os.ReadFile(overridePath)
	if err != nil {
		return fmt.Errorf("failed to read KPS workload override: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, content, map[string]string{
		"namespace": workspaceNSName,
	}, environment.WorkloadClusterTarget)
}

// deployKPSOnWorkload deploys kube-prometheus-stack on the workload cluster.
func (o *openCost) deployKPSOnWorkload(ctx context.Context, env *environment.Env) error {
	if err := o.applyKPSWorkloadOverride(ctx, env); err != nil {
		return fmt.Errorf("failed to apply KPS workload override: %w", err)
	}

	appPath, err := absolutePathTo(constants.KubePrometheusStack)
	if err != nil {
		return err
	}

	// Apply the helmrelease directory
	helmReleasePath := filepath.Join(appPath, "helmrelease")
	return env.ApplyKustomizations(ctx, helmReleasePath, map[string]string{
		"releaseName":        "kube-prometheus-stack",
		"appVersion":         "app-version",
		"releaseNamespace":   workspaceNSName,
		"workspaceNamespace": workspaceNSName,
	}, environment.WorkloadClusterTarget)
}

// getWorkloadNodeIP gets the internal IP of the workload cluster node.
// This IP is routable from the management cluster since both Kind clusters share the same Docker network.
func (o *openCost) getWorkloadNodeIP(ctx context.Context, client genericClient.Client) (string, error) {
	nodeList := &corev1.NodeList{}
	if err := client.List(ctx, nodeList); err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodeList.Items {
		if node.Name == "workload-worker" {
			for _, addr := range node.Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					return addr.Address, nil
				}
			}
		}
	}

	return "", fmt.Errorf("workload-worker node not found or has no internal IP")
}

// deployKPSOnManagement deploys kube-prometheus-stack on the management cluster.
func (o *openCost) deployKPSOnManagement(ctx context.Context, env *environment.Env) error {
	if err := o.applyKPSManagementOverride(ctx, env); err != nil {
		return fmt.Errorf("failed to apply KPS management override: %w", err)
	}

	appPath, err := absolutePathTo(constants.KubePrometheusStack)
	if err != nil {
		return err
	}

	// Apply the helmrelease directory
	helmReleasePath := filepath.Join(appPath, "helmrelease")
	return env.ApplyKustomizations(ctx, helmReleasePath, map[string]string{
		"releaseName":        "kube-prometheus-stack",
		"appVersion":         "app-version",
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
}

// applyKPSManagementOverride applies the KPS override ConfigMap on the management cluster.
func (o *openCost) applyKPSManagementOverride(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	overridePath := filepath.Join(testDataPath, "opencost", "kps-management-override.yaml")
	content, err := os.ReadFile(overridePath)
	if err != nil {
		return fmt.Errorf("failed to read KPS management override: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, content, map[string]string{
		"namespace": kommanderNamespace,
	})
}

// createThanosStoresConfigMap creates the ConfigMap for Thanos file-based service discovery.
func (o *openCost) createThanosStoresConfigMap(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	storesPath := filepath.Join(testDataPath, "opencost", "thanos-stores.yaml")
	content, err := os.ReadFile(storesPath)
	if err != nil {
		return fmt.Errorf("failed to read Thanos stores ConfigMap: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, content, map[string]string{
		"namespace":            kommanderNamespace,
		"workloadNodeIP":       o.workloadNodeIP,
		"managementServiceUrl": o.managementServiceUrl,
	})
}

// deployThanosOnManagement deploys Thanos on the management cluster with TLS disabled.
func (o *openCost) deployThanosOnManagement(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	// Apply the custom Thanos config defaults (TLS disabled)
	configPath := filepath.Join(testDataPath, "opencost", "thanos-config-defaults.yaml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read Thanos config: %w", err)
	}

	if err := env.ApplyYAMLFileRaw(ctx, configContent, map[string]string{
		"releaseName":      "thanos",
		"appVersion":       "app-version",
		"namespace":        kommanderNamespace,
		"releaseNamespace": kommanderNamespace,
	}); err != nil {
		return fmt.Errorf("failed to apply Thanos config: %w", err)
	}

	// Apply the custom HelmRelease (without Certificate resource)
	helmReleasePath := filepath.Join(testDataPath, "opencost", "thanos-helmrelease.yaml")
	helmReleaseContent, err := os.ReadFile(helmReleasePath)
	if err != nil {
		return fmt.Errorf("failed to read Thanos HelmRelease: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, helmReleaseContent, map[string]string{
		"releaseName":      "thanos",
		"appVersion":       "app-version",
		"namespace":        kommanderNamespace,
		"releaseNamespace": kommanderNamespace,
	})
}

// deployCentralizedOpenCost deploys Centralized OpenCost on the management cluster.
func (o *openCost) deployCentralizedOpenCost(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(constants.CentralizedOpenCost)
	if err != nil {
		return err
	}

	// Apply the release directory
	releasePath := filepath.Join(appPath, "release")
	return env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseName":        "centralized-opencost",
		"appVersion":         "app-version",
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
}

// GetWorkloadNodeIP returns the Node IP of the workload cluster.
// This can be used by tests to verify connectivity via NodePort.
func (o *openCost) GetWorkloadNodeIP() string {
	return o.workloadNodeIP
}

// GetManagementServiceUrl returns the service URL of the management cluster's Prometheus.
// This can be used by tests to verify the management Prometheus is registered as a Thanos store.
func (o *openCost) GetManagementServiceUrl() string {
	return o.managementServiceUrl
}
