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

type karma struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k karma) Name() string {
	return constants.Karma
}

var _ scenarios.AppScenario = (*karma)(nil)

func NewKarma() *karma {
	appPath, _ := absolutePathTo(constants.Karma)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.Karma)
	return &karma{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (k karma) Install(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathCurrentVersion)
	return err
}

func (k karma) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathPreviousVersion)

	return err
}

func (k karma) InstallDependency(ctx context.Context, env *environment.Env, depAppName string) error {
	appPath, err := absolutePathTo(depAppName)
	if err != nil {
		return err
	}
	err = k.install(ctx, env, appPath)

	return err
}

func (k karma) install(ctx context.Context, env *environment.Env, appPath string) error {
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
	err := env.ApplyKustomizations(ctx, appPath, map[string]string{
		"appName":            "app-name" + filepath.Base(appPath),
		"releaseName":        "app-deployment-name" + filepath.Base(appPath),
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (k karma) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
