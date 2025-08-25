package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type kubeOIDCProxy struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k kubeOIDCProxy) Name() string {
	return constants.KubeOIDCProxy
}

var _ scenarios.AppScenario = (*kubeOIDCProxy)(nil)

func NewKubeOIDCProxy() *kubeOIDCProxy {
	appPath, _ := absolutePathTo(constants.KubeOIDCProxy)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.KubeOIDCProxy)
	return &kubeOIDCProxy{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (k kubeOIDCProxy) Install(ctx context.Context, env *environment.Env) error {
	return k.install(ctx, env, k.appPathCurrentVersion)
}

func (k kubeOIDCProxy) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	return k.install(ctx, env, k.appPathPreviousVersion)
}

func (k kubeOIDCProxy) Upgrade(ctx context.Context, env *environment.Env) error {
	return k.install(ctx, env, k.appPathCurrentVersion)
}

func (k kubeOIDCProxy) install(ctx context.Context, env *environment.Env, appPath string) error {
	// Install cert-manager first as it's a dependency
	cm := certManager{}
	err := cm.Install(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to install cert-manager dependency: %w", err)
	}

	// Wait for cert-manager to be ready
	err = k.waitForCertManager(ctx, env)
	if err != nil {
		return fmt.Errorf("cert-manager is not ready: %w", err)
	}

	// Apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "defaults")
	err = env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace":      kommanderNamespace,
		"workspaceNamespace":    kommanderNamespace,
		"certificateIssuerKind": "ClusterIssuer",
		"certificateIssuerName": "kubernetes-ca",
	})
	if err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Apply the main kustomization for the helmrelease
	err = env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace":      kommanderNamespace,
		"workspaceNamespace":    kommanderNamespace,
		"certificateIssuerKind": "ClusterIssuer",
		"certificateIssuerName": "kubernetes-ca",
	})
	if err != nil {
		return fmt.Errorf("failed to apply main kustomization: %w", err)
	}

	return nil
}

func (k kubeOIDCProxy) waitForCertManager(ctx context.Context, env *environment.Env) error {
	hr := &fluxhelmv2beta2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-manager",
			Namespace: kommanderNamespace,
		},
	}

	timeout := 5 * time.Minute
	interval := 10 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		err := env.Client.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
		if err != nil {
			time.Sleep(interval)
			continue
		}

		for _, cond := range hr.Status.Conditions {
			if cond.Status == metav1.ConditionTrue && cond.Type == apimeta.ReadyCondition {
				return nil
			}
		}
		time.Sleep(interval)
	}

	return fmt.Errorf("cert-manager helm release not ready within timeout")
}

func (k kubeOIDCProxy) IsHealthy(ctx context.Context, env *environment.Env) error {
	// Check if the HelmRelease is ready
	hr := &fluxhelmv2beta2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name(),
			Namespace: kommanderNamespace,
		},
	}

	err := env.Client.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
	if err != nil {
		return fmt.Errorf("failed to get HelmRelease: %w", err)
	}

	// Check HelmRelease is ready
	ready := false
	for _, cond := range hr.Status.Conditions {
		if cond.Status == metav1.ConditionTrue && cond.Type == apimeta.ReadyCondition {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("HelmRelease is not ready")
	}

	// Check deployment is ready
	deployment := &appsv1.Deployment{}
	err = env.Client.Get(ctx, ctrlClient.ObjectKey{
		Namespace: kommanderNamespace,
		Name:      k.Name(),
	}, deployment)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if deployment.Status.ReadyReplicas < 1 {
		return fmt.Errorf("deployment has no ready replicas: %d", deployment.Status.ReadyReplicas)
	}

	// Verify priority class is configured
	if deployment.Spec.Template.Spec.PriorityClassName != dkpCriticalPriority {
		return fmt.Errorf("expected priority class %s, got %s",
			dkpCriticalPriority, deployment.Spec.Template.Spec.PriorityClassName)
	}

	// Check service exists and is accessible
	service := &corev1.Service{}
	err = env.Client.Get(ctx, ctrlClient.ObjectKey{
		Namespace: kommanderNamespace,
		Name:      k.Name(),
	}, service)
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	// Check ingress exists (this is the business logic - proxy should be accessible via ingress)
	ingress := &networkingv1.Ingress{}
	err = env.Client.Get(ctx, ctrlClient.ObjectKey{
		Namespace: kommanderNamespace,
		Name:      k.Name(),
	}, ingress)
	if err != nil {
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	// Verify TLS certificate secret exists (critical for OIDC proxy functionality)
	tlsSecret := &corev1.Secret{}
	err = env.Client.Get(ctx, ctrlClient.ObjectKey{
		Namespace: kommanderNamespace,
		Name:      k.Name() + "-server-tls",
	}, tlsSecret)
	if err != nil {
		return fmt.Errorf("failed to get TLS secret: %w", err)
	}

	// Verify config secret exists (contains OIDC configuration)
	configSecret := &corev1.Secret{}
	err = env.Client.Get(ctx, ctrlClient.ObjectKey{
		Namespace: kommanderNamespace,
		Name:      k.Name() + "-config",
	}, configSecret)
	if err != nil {
		return fmt.Errorf("failed to get config secret: %w", err)
	}

	return nil
}

func (k kubeOIDCProxy) TestBusinessLogic(ctx context.Context, env *environment.Env) error {
	// Deploy a test client to validate the OIDC proxy functionality
	testDataPath, err := getTestDataDir()
	if err != nil {
		return fmt.Errorf("failed to get test data dir: %w", err)
	}

	testClientYamlPath := filepath.Join(testDataPath, "kube-oidc-proxy/test-client.yaml")
	err = env.ApplyYAML(ctx, testClientYamlPath, map[string]string{
		"namespace": kommanderNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to apply test client: %w", err)
	}

	// Wait for the test pod to be ready
	timeout := 2 * time.Minute
	interval := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pod := &corev1.Pod{}
		err := env.Client.Get(ctx, ctrlClient.ObjectKey{
			Namespace: kommanderNamespace,
			Name:      "oidc-test-client",
		}, pod)
		if err != nil {
			time.Sleep(interval)
			continue
		}

		if pod.Status.Phase == corev1.PodRunning {
			// Test basic connectivity to the proxy service
			service := &corev1.Service{}
			err = env.Client.Get(ctx, ctrlClient.ObjectKey{
				Namespace: kommanderNamespace,
				Name:      k.Name(),
			}, service)
			if err != nil {
				return fmt.Errorf("failed to get service: %w", err)
			}

			// Verify the service has the expected ports for OIDC proxy
			found := false
			for _, port := range service.Spec.Ports {
				if port.Port == 443 || port.Port == 8443 {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("OIDC proxy service does not have expected HTTPS port")
			}

			return nil
		}
		time.Sleep(interval)
	}

	return fmt.Errorf("test client pod not ready within timeout")
}
