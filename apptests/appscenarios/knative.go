package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
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

// InstallIstioHelmDependency installs istio-helm which is required by Knative
func (k knative) InstallIstioHelmDependency(ctx context.Context, env *environment.Env) error {
	istioHelmPath, err := absolutePathTo("istio-helm")
	if err != nil {
		return fmt.Errorf("failed to get path for istio-helm: %w", err)
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

	// Apply pre-install if it exists
	preInstallPath := filepath.Join(istioHelmPath, "pre-install")
	if _, err := os.Stat(preInstallPath); err == nil {
		err := env.ApplyYAML(ctx, preInstallPath, map[string]string{
			"releaseName":      "istio-helm",
			"appVersion":       "app-version-istio-helm",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return fmt.Errorf("failed to apply pre-install for istio-helm: %w", err)
		}
	}

	// Apply istio-helm-gateway-namespace
	gatewayNsPath := filepath.Join(istioHelmPath, "istio-helm-gateway-namespace")
	if _, err := os.Stat(gatewayNsPath); err == nil {
		err := env.ApplyYAML(ctx, gatewayNsPath, map[string]string{
			"releaseName":      "istio-helm",
			"appVersion":       "app-version-istio-helm",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return fmt.Errorf("failed to apply gateway namespace for istio-helm: %w", err)
		}
	}

	// Install istio-helm main resources
	err = env.ApplyKustomizations(ctx, istioHelmPath, map[string]string{
		"releaseName":      "istio-helm",
		"appVersion":       "app-version-istio-helm",
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to apply istio-helm: %w", err)
	}

	return nil
}
