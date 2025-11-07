package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	traefikv1a1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type traefik struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (t traefik) Name() string {
	return constants.Traefik
}

var _ scenarios.AppScenario = (*traefik)(nil)

func NewTraefik() *traefik {
	appPath, _ := absolutePathTo(constants.Traefik)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.Traefik)
	return &traefik{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func setupTraefikSchema(env *environment.Env) error {
	scheme := flux.NewScheme()
	err := traefikv1a1.AddToScheme(scheme)
	if err != nil {
		return err
	}
	c, err := genericCLient.New(env.K8sClient.Config(), genericCLient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return err
	}
	env.SetClient(c)
	return nil
}

func (t traefik) Install(ctx context.Context, env *environment.Env) error {
	err := setupTraefikSchema(env)
	if err != nil {
		return err
	}
	err = t.install(ctx, env, t.appPathCurrentVersion)

	return err
}

func (t traefik) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := t.install(ctx, env, t.appPathPreviousVersion)

	return err
}

func (t traefik) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"releaseNamespace": kommanderNamespace,
			"tfaName":          "traefik-forward-auth-mgmt",
		})
		if err != nil {
			return err
		}
	}
	traefikCMName := "traefik-overrides"
	err := t.applyTraefikOverrideCM(ctx, env, traefikCMName)
	if err != nil {
		return err
	}

	// apply the rest of kustomizations
	traefikDir := filepath.Join(appPath, "traefik")
	if _, err := os.Stat(traefikDir); !os.IsNotExist(err) {
		// Find the correct versioned path for gateway-api-crds
		gatewayCRDsPath, err := absolutePathTo("gateway-api-crds") // Ensure the correct version is used
		if err != nil {
			return fmt.Errorf("failed to get path for gateway-api-crds: %w", err)
		}

		// Apply defaults for gateway-api-crds
		defaultKustomization := filepath.Join(gatewayCRDsPath, "/defaults")
		if _, err := os.Stat(defaultKustomization); err == nil {
			err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
				"releaseNamespace": kommanderNamespace,
			})
			if err != nil {
				return fmt.Errorf("failed to apply defaults for gateway-api-crds: %w", err)
			}
		}
		// Install gateway-api-crds
		err = env.ApplyKustomizations(ctx, gatewayCRDsPath, map[string]string{
			"appName":          "app-name-gateway-api-crds",
			"releaseName":      "gateway-api-crds",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return fmt.Errorf("failed to apply gateway-api CRDs: %w", err)
		}

		// If the traefik directory exists, apply both `crds` and `traefik` subdirectories
		for _, dir := range []string{"crds", "traefik"} {
			subDir := filepath.Join(appPath, dir)
			err := env.ApplyKustomizations(ctx, subDir, map[string]string{
				"appName":            "app-name-gateway" + dir,
				"releaseName":        "app-deployment-name",
				"releaseNamespace":   kommanderNamespace,
				"workspaceNamespace": kommanderNamespace,
			})
			if err != nil {
				return err
			}
		}
	}

	// If the `traefik` directory doesn't exist, apply the default (root) kustomizations
	return env.ApplyKustomizations(ctx, appPath, map[string]string{
		"appName":            "app-name-traefik",
		"releaseName":        "app-deployment-name",
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
}

func (t traefik) applyTraefikOverrideCM(ctx context.Context, env *environment.Env, cmName string) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}
	cmPath := filepath.Join(testDataPath, "traefik", "override-cm.yaml")
	err = env.ApplyYAML(ctx, cmPath, map[string]string{
		"name":      cmName,
		"namespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}

func (t traefik) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
