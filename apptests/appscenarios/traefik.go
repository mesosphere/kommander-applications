package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
)

type traefik struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (t traefik) Name() string {
	return constants.Traefik
}

var _ AppScenario = (*traefik)(nil)

func NewTraefik() *traefik {
	appPath, _ := absolutePathTo(constants.Traefik)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.Traefik)
	return &traefik{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (t traefik) Install(ctx context.Context, env *environment.Env) error {
	err := t.install(ctx, env, t.appPathCurrentVersion)

	return err
}

func (t traefik) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := t.install(ctx, env, t.appPathPreviousVersion)

	return err
}

func (t traefik) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace": kommanderNamespace,
		"tfaName":          "traefik-forward-auth-mgmt",
	})
	if err != nil {
		return err
	}

	// Find the correct versioned path for gateway-api-crds
	gatewayCRDsPath, err := absolutePathTo("gateway-api-crds") // Ensure the correct version is used
	if err != nil {
		return fmt.Errorf("failed to get path for gateway-api-crds: %w", err)
	}

	// Apply defaults for gateway-api-crds
	err = env.ApplyKustomizations(ctx, filepath.Join(gatewayCRDsPath, "/defaults"), map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to apply defaults for gateway-api-crds: %w", err)
	}

	// Install gateway-api-crds
	err = env.ApplyKustomizations(ctx, gatewayCRDsPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return fmt.Errorf("failed to apply gateway-api CRDs: %w", err)
	}
	// apply the rest of kustomizations
	traefikDir := filepath.Join(appPath, "traefik")
	if _, err := os.Stat(traefikDir); !os.IsNotExist(err) {
		// If the traefik directory exists, apply both `crds` and `traefik` subdirectories
		for _, dir := range []string{"crds", "traefik"} {
			subDir := filepath.Join(appPath, dir)
			err := env.ApplyKustomizations(ctx, subDir, map[string]string{
				"releaseNamespace":   kommanderNamespace,
				"workspaceNamespace": kommanderNamespace,
			})
			if err != nil {
				return err
			}
		}
	} else {
		// If the `traefik` directory doesn't exist, apply the default (root) kustomizations
		err = env.ApplyKustomizations(ctx, appPath, map[string]string{
			"releaseNamespace":   kommanderNamespace,
			"workspaceNamespace": kommanderNamespace,
		})
		if err != nil {
			return err
		}
	}

	traefikCMName := "traefik-overrides"
	err = t.applyTraefikOverrideCM(ctx, env, traefikCMName)
	if err != nil {
		return err
	}

	hr := &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.Traefik,
			Namespace: kommanderNamespace,
		},
	}

	genericClient, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("could not create the generic client: %w", err)
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = controllerruntime.CreateOrUpdate(ctx, genericClient, hr, func() error {
			hr.Spec.ValuesFrom = append(hr.Spec.ValuesFrom, fluxhelmv2beta2.ValuesReference{
				Kind: "ConfigMap",
				Name: traefikCMName,
			})
			return nil
		})
		return err
	})

	return err
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
