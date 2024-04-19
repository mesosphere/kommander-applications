package appscenarios

import (
	"context"
	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"path/filepath"
)

type karma struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k karma) Name() string {
	return constants.Karma
}

var _ AppScenario = (*karma)(nil)

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

func (r karma) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := r.install(ctx, env, r.appPathPreviousVersion)

	return err
}

func (r karma) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}
	// apply the rest of kustomizations
	err = env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}
