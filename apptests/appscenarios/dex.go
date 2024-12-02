package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type dex struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (d dex) Name() string {
	return constants.Dex
}

var _ AppScenario = (*dex)(nil)

func NewDex() *dex {
	appPath, _ := absolutePathTo(constants.Dex)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.Dex)
	return &dex{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (d dex) Install(ctx context.Context, env *environment.Env) error {
	err := d.install(ctx, env, d.appPathCurrentVersion)
	fmt.Println("*******Install error **********", err)
	return err
}

func (d dex) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := d.install(ctx, env, d.appPathPreviousVersion)

	return err
}

func (d dex) InstallDependency(ctx context.Context, env *environment.Env, depAppName string) error {
	appPath, err := absolutePathTo(depAppName)
	if err != nil {
		return err
	}
	err = d.install(ctx, env, appPath)

	return err
}

func (d dex) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")

	fmt.Println("*********Deafult kuzt", defaultKustomizations)
	err := env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	fmt.Println("Error *************", err)
	if err != nil {
		return err
	}
	// apply the rest of kustomizations
	err = env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	fmt.Println("*********************** apply the rest of kustomizations err ", err)
	if err != nil {
		return err
	}

	return err
}
