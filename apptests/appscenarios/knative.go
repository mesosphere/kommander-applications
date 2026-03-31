package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type knative struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k knative) Name() string {
	return constants.Knative
}

var _ scenarios.AppScenario = (*knative)(nil)

func NewKnative() *knative {
	appPath, _ := absolutePathTo(constants.Knative)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.Knative)
	return &knative{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (k knative) Install(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathCurrentVersion)
	return err
}

func (k knative) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathPreviousVersion)
	return err
}

func (k knative) Upgrade(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathCurrentVersion)
	return err
}

func (k knative) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"appVersion":       "app-version-knative",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return err
		}
	}

	// Apply the helmrelease kustomizations
	err := env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseName":      "knative",
		"appVersion":       "app-version-knative",
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}

// InstallIstioHelmDependency installs istio-helm which is required by Knative.
// It applies the helmrelease subdirectory directly rather than the top-level
// Flux Kustomization wrappers, which reference a GitRepository "management"
// that does not exist in test clusters.
func (k knative) InstallIstioHelmDependency(ctx context.Context, env *environment.Env) error {
	istioHelmPath, err := absolutePathTo("istio-helm")
	if err != nil {
		return fmt.Errorf("failed to get path for istio-helm: %w", err)
	}

	substMap := map[string]string{
		"releaseName":      "istio-helm",
		"appVersion":       "app-version-istio-helm",
		"releaseNamespace": kommanderNamespace,
		"caIssuerName":     "kommander-ca",
	}

	// Apply defaults for istio-helm
	defaultKustomization := filepath.Join(istioHelmPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"appVersion":       "app-version-istio-helm",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return fmt.Errorf("failed to apply defaults for istio-helm: %w", err)
		}
	}

	// Apply pre-install resources directly
	preInstallPath := filepath.Join(istioHelmPath, "pre-install")
	if _, err := os.Stat(preInstallPath); err == nil {
		err := env.ApplyYAML(ctx, preInstallPath, substMap)
		if err != nil {
			return fmt.Errorf("failed to apply pre-install for istio-helm: %w", err)
		}
	}

	// Apply istio-helm-gateway-namespace directly
	gatewayNsPath := filepath.Join(istioHelmPath, "istio-helm-gateway-namespace")
	if _, err := os.Stat(gatewayNsPath); err == nil {
		err := env.ApplyYAML(ctx, gatewayNsPath, substMap)
		if err != nil {
			return fmt.Errorf("failed to apply gateway namespace for istio-helm: %w", err)
		}
	}

	// The istiod chart creates ServiceMonitor resources; install the CRD so
	// Helm can render the manifests in test clusters that lack kube-prometheus-stack.
	if err := installServiceMonitorCRD(ctx, env); err != nil {
		return fmt.Errorf("failed to install ServiceMonitor CRD: %w", err)
	}

	// Create config overrides to disable features that depend on components
	// not present in test clusters (cert-manager CA issuer, etc.).
	if err := createIstioHelmTestOverrides(ctx, env); err != nil {
		return fmt.Errorf("failed to create istio-helm test overrides: %w", err)
	}

	// Apply the helmrelease subdirectory directly (OCIRepositories, HelmReleases,
	// ConfigMap) instead of the top-level kustomization which creates Flux
	// Kustomization wrappers that require a GitRepository source.
	helmreleasePath := filepath.Join(istioHelmPath, "helmrelease")
	err = env.ApplyKustomizations(ctx, helmreleasePath, substMap)
	if err != nil {
		return fmt.Errorf("failed to apply istio-helm helmrelease: %w", err)
	}

	// The istio-helm-base HelmRelease depends on kube-prometheus-stack, which is
	// not installed in test clusters. Remove that dependency so the install can
	// proceed without it.
	if err := k.removeIstioHelmDependsOn(ctx, env); err != nil {
		return fmt.Errorf("failed to patch istio-helm dependencies: %w", err)
	}

	return nil
}

// createIstioHelmTestOverrides creates the optional istio-helm-config-overrides
// ConfigMap to disable features that require components not present in test
// clusters (cert-manager CA issuer for the cacert-job, etc.).
func createIstioHelmTestOverrides(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-helm-config-overrides",
			Namespace: kommanderNamespace,
		},
		Data: map[string]string{
			"values.yaml": "security:\n  enabled: false\n",
		},
	}

	err = client.Create(ctx, cm)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// installServiceMonitorCRD creates a minimal ServiceMonitor CRD so the istiod
// Helm chart can render without kube-prometheus-stack being installed.
func installServiceMonitorCRD(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	preserveUnknown := true
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "servicemonitors.monitoring.coreos.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "monitoring.coreos.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "servicemonitors",
				Singular: "servicemonitor",
				Kind:     "ServiceMonitor",
				ListKind: "ServiceMonitorList",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &preserveUnknown,
						},
					},
				},
			},
		},
	}

	err = client.Create(ctx, crd)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// removeIstioHelmDependsOn patches the istio-helm-base HelmRelease to remove
// the dependsOn on kube-prometheus-stack, which is not present in test clusters.
func (k knative) removeIstioHelmDependsOn(ctx context.Context, env *environment.Env) error {
	genericClient, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return err
	}

	hr := &fluxhelmv2.HelmRelease{}
	err = genericClient.Get(ctx, ctrlClient.ObjectKey{
		Name:      "istio-helm-base",
		Namespace: kommanderNamespace,
	}, hr)
	if err != nil {
		return fmt.Errorf("could not get istio-helm-base HelmRelease: %w", err)
	}

	filtered := make([]fluxhelmv2.DependencyReference, 0, len(hr.Spec.DependsOn))
	for _, dep := range hr.Spec.DependsOn {
		if dep.Name != "kube-prometheus-stack" {
			filtered = append(filtered, dep)
		}
	}
	hr.Spec.DependsOn = filtered

	// Use MergePatch via Update to write back the cleared dependsOn
	hr.ManagedFields = nil
	hr.TypeMeta = metav1.TypeMeta{
		Kind:       fluxhelmv2.HelmReleaseKind,
		APIVersion: fluxhelmv2.GroupVersion.String(),
	}
	return genericClient.Update(ctx, hr)
}
