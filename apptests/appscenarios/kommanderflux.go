package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type kommanderFlux struct{}

func (r kommanderFlux) Name() string {
	return constants.Flux
}

var _ scenarios.AppScenario = (*reloader)(nil)

func (r kommanderFlux) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	// Bootstrap flux from applications/kommander-flux dir
	err = r.install(ctx, env, filepath.Join(appPath, ".."))
	if err != nil {
		return err
	}

	return err
}

func (r kommanderFlux) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
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

func (r kommanderFlux) Upgrade(ctx context.Context, env *environment.Env) error {
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

func (r kommanderFlux) install(ctx context.Context, env *environment.Env, appPath string) error {
	fmt.Println("Installing application from path", appPath)
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
