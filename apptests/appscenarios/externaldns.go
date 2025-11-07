package appscenarios

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type externalDns struct{}

func (r externalDns) Name() string {
	return constants.ExternalDns
}

var _ scenarios.AppScenario = (*reloader)(nil)

func (r externalDns) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (r externalDns) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (r externalDns) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (r externalDns) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"releaseNamespace":   kommanderNamespace,
			"workspaceNamespace": kommanderNamespace,
		})
		if err != nil {
			return err
		}
	}
	// apply the kustomization for the helmrelease
	releasePath := filepath.Join(appPath, "/")
	err := env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseName":      "app-deployment-name",
		"appName":          "app-name",
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}
