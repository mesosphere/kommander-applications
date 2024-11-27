package appscenarios

import (
	"context"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type dex struct{}

func (r dex) Name() string {
	return constants.Dex
}

var _ AppScenario = (*dex)(nil)

func (r dex) Install(ctx context.Context, env *environment.Env) error {
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

func (r dex) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
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

func (r dex) Upgrade(ctx context.Context, env *environment.Env) error {
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

func (r dex) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the helmrelease
	releasePath := filepath.Join(appPath, "/")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}
