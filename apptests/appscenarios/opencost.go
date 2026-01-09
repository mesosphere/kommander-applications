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

// openCost implements multi-cluster OpenCost deployment scenario.
// This deploys:
// - Workload cluster: kube-prometheus-stack (with NodePort) + opencost
// - Management cluster: thanos (pointing to workload) + centralized-opencost
type openCost struct {
	// workloadNodeIP stores the Node IP of the workload cluster for Thanos to connect via NodePort
	workloadNodeIP string
}

var _ scenarios.AppScenario = (*openCost)(nil)

func NewOpenCost() *openCost {
	return &openCost{}
}

func (o *openCost) Name() string {
	return constants.OpenCost
}

// Install deploys the multi-cluster OpenCost setup.
// Workload cluster: KPS + OpenCost
// Management cluster: Thanos + Centralized OpenCost
func (o *openCost) Install(ctx context.Context, env *environment.Env) error {
	nodeIP, err := o.deployWorkloadClusterApps(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to deploy workload cluster apps: %w", err)
	}
	o.workloadNodeIP = nodeIP

	if err := o.deployManagementClusterApps(ctx, env); err != nil {
		return fmt.Errorf("failed to deploy management cluster apps: %w", err)
	}

	return nil
}

func (o *openCost) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("InstallPreviousVersion is not implemented for multi-cluster OpenCost")
}

func (o *openCost) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("Upgrade is not implemented for multi-cluster OpenCost")
}

// deployWorkloadClusterApps deploys KPS and OpenCost on the workload cluster.
// Returns the Node IP for Thanos to connect via NodePort.
func (o *openCost) deployWorkloadClusterApps(ctx context.Context, env *environment.Env) (string, error) {
	// Step 1: Apply KPS override ConfigMap (sets NodePort type and exposes Thanos gRPC port)
	if err := o.applyKPSWorkloadOverride(ctx, env); err != nil {
		return "", fmt.Errorf("failed to apply KPS workload override: %w", err)
	}

	// Step 2: Deploy kube-prometheus-stack on workload cluster
	if err := o.deployKPSOnWorkload(ctx, env); err != nil {
		return "", fmt.Errorf("failed to deploy KPS on workload cluster: %w", err)
	}

	// Step 3: Get workload cluster node IP for Thanos to connect via NodePort
	nodeIP, err := o.getWorkloadNodeIP(ctx, env.WorkloadClient)
	if err != nil {
		return "", fmt.Errorf("failed to get workload node IP: %w", err)
	}

	// Step 4: Deploy OpenCost pre-install job (creates cluster-info-configmap)
	if err := o.deployOpenCostPreInstall(ctx, env, env.WorkloadClient); err != nil {
		return "", fmt.Errorf("failed to deploy OpenCost pre-install on workload cluster: %w", err)
	}

	// Step 5: Deploy OpenCost on workload cluster
	if err := o.deployOpenCostOnWorkload(ctx, env); err != nil {
		return "", fmt.Errorf("failed to deploy OpenCost on workload cluster: %w", err)
	}

	return nodeIP, nil
}

// deployManagementClusterApps deploys Thanos, and Centralized OpenCost on the management cluster.
func (o *openCost) deployManagementClusterApps(ctx context.Context, env *environment.Env) error {
	if err := o.createThanosStoresConfigMap(ctx, env); err != nil {
		return fmt.Errorf("failed to create Thanos stores ConfigMap: %w", err)
	}

	if err := o.deployThanosOnManagement(ctx, env); err != nil {
		return fmt.Errorf("failed to deploy Thanos on management cluster: %w", err)
	}

	if err := o.deployCentralizedOpenCost(ctx, env); err != nil {
		return fmt.Errorf("failed to deploy Centralized OpenCost: %w", err)
	}

	return nil
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
	appPath, err := absolutePathTo(constants.KubePrometheusStack)
	if err != nil {
		return err
	}

	// Apply the helmrelease directory
	helmReleasePath := filepath.Join(appPath, "helmrelease")
	return env.ApplyKustomizations(ctx, helmReleasePath, map[string]string{
		"releaseName":        "app-name",
		"appVersion":         "app-version",
		"releaseNamespace":   workspaceNSName,
		"workspaceNamespace": workspaceNSName,
	}, environment.WorkloadClusterTarget)
}

// getWorkloadNodeIP gets the internal IP of a workload cluster node.
// This IP is routable from the management cluster since both Kind clusters share the same Docker network.
func (o *openCost) getWorkloadNodeIP(ctx context.Context, client genericClient.Client) (string, error) {
	nodeList := &corev1.NodeList{}
	if err := client.List(ctx, nodeList); err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodeList.Items) == 0 {
		return "", fmt.Errorf("no nodes found in workload cluster")
	}

	// Get the internal IP of the first node
	for _, addr := range nodeList.Items[0].Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return "", fmt.Errorf("no internal IP found for workload cluster node")
}

// deployOpenCostPreInstall applies the OpenCost pre-install job to create cluster-info-configmap.
func (o *openCost) deployOpenCostPreInstall(ctx context.Context, env *environment.Env, client genericClient.Client) error {
	appPath, err := absolutePathTo(constants.OpenCost)
	if err != nil {
		return err
	}

	preInstallPath := filepath.Join(appPath, "pre-install")
	if _, err := os.Stat(preInstallPath); os.IsNotExist(err) {
		// Pre-install directory doesn't exist, skip
		return nil
	}

	return env.ApplyKustomizations(ctx, preInstallPath, map[string]string{
		"releaseName":              "app-name",
		"appVersion":               "app-version",
		"releaseNamespace":         kommanderNamespace,
		"kubetoolsImageRepository": "docker.io/mesosphere/kubectl",
		"kubetoolsImageTag":        kubetoolsImageTag,
	}, environment.WorkloadClusterTarget)
}

// deployOpenCostOnWorkload deploys OpenCost on the workload cluster.
func (o *openCost) deployOpenCostOnWorkload(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(constants.OpenCost)
	if err != nil {
		return err
	}

	// Apply the release directory
	releasePath := filepath.Join(appPath, "release")
	return env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseName":        "app-name",
		"appVersion":         "app-version",
		"releaseNamespace":   workspaceNSName,
		"workspaceNamespace": workspaceNSName,
	}, environment.WorkloadClusterTarget)
}

// deployKPSOnManagement deploys kube-prometheus-stack on the management cluster.
func (o *openCost) deployKPSOnManagement(ctx context.Context, env *environment.Env) error {
	// Apply KPS override ConfigMap (emptyDir instead of PVC for Kind testing)
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
		"releaseName":        "app-name",
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
		"namespace":      kommanderNamespace,
		"workloadNodeIP": o.workloadNodeIP,
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
		"releaseName":      "app-name",
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
		"releaseName":      "app-name",
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
		"releaseName":        "app-name",
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
