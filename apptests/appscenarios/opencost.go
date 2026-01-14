package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
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
	}, environment.WithTarget(environment.WorkloadClusterTarget))
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
func (o *openCost) getWorkloadNodeIP(ctx context.Context, client ctrlClient.Client) (string, error) {
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
	appPath, err := absolutePathTo(constants.Thanos)
	if err != nil {
		return fmt.Errorf("failed to get Thanos app path: %w", err)
	}

	// Apply the production config defaults from applications/thanos
	configPath := filepath.Join(appPath, "helmrelease", "cm.yaml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read Thanos config: %w", err)
	}

	if err := env.ApplyYAMLFileRaw(ctx, configContent, map[string]string{
		"releaseName":      "thanos",
		"appVersion":       "app-version",
		"releaseNamespace": kommanderNamespace,
	}); err != nil {
		return fmt.Errorf("failed to apply Thanos config: %w", err)
	}

	// Apply the override ConfigMap to disable TLS for testing
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	overridePath := filepath.Join(testDataPath, "opencost", "thanos-overrides.yaml")
	overrideContent, err := os.ReadFile(overridePath)
	if err != nil {
		return fmt.Errorf("failed to read Thanos overrides: %w", err)
	}

	if err := env.ApplyYAMLFileRaw(ctx, overrideContent, map[string]string{
		"namespace": kommanderNamespace,
	}); err != nil {
		return fmt.Errorf("failed to apply Thanos overrides: %w", err)
	}

	// Apply the production HelmRelease, skipping Certificate and ClusterRole resources
	// Certificate requires cert-manager/ClusterIssuer which we don't have in the test environment
	helmReleasePath := filepath.Join(appPath, "helmrelease", "thanos.yaml")
	helmReleaseContent, err := os.ReadFile(helmReleasePath)
	if err != nil {
		return fmt.Errorf("failed to read Thanos HelmRelease: %w", err)
	}

	if err := env.ApplyYAMLFileRaw(ctx, helmReleaseContent, map[string]string{
		"releaseName":      "thanos",
		"appVersion":       "app-version",
		"releaseNamespace": kommanderNamespace,
	}, environment.WithKindsToSkip([]string{"Certificate", "ClusterRole"})); err != nil {
		return fmt.Errorf("failed to apply Thanos HelmRelease: %w", err)
	}

	return o.patchThanosHelmReleaseWithOverride(ctx, env)
}

// patchThanosHelmReleaseWithOverride patches the Thanos HelmRelease to include the override ConfigMap.
func (o *openCost) patchThanosHelmReleaseWithOverride(ctx context.Context, env *environment.Env) error {
	hr := &fluxhelmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2.HelmReleaseKind,
			APIVersion: fluxhelmv2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "thanos",
			Namespace: kommanderNamespace,
		},
	}

	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("could not create the generic client: %w", err)
	}

	if err := client.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr); err != nil {
		return fmt.Errorf("could not get the HelmRelease: %w", err)
	}

	hr.Spec.ValuesFrom = append(hr.Spec.ValuesFrom, fluxhelmv2.ValuesReference{
		Kind: "ConfigMap",
		Name: "thanos-overrides",
	})

	if err := client.Update(ctx, hr); err != nil {
		return fmt.Errorf("could not update the HelmRelease: %w", err)
	}

	return nil
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
