package appscenarios

import (
	"context"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type gatekeeper struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

var _ AppScenario = (*gatekeeper)(nil)

func (g gatekeeper) Install(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathCurrentVersion)
	return err
}

func (g gatekeeper) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathPreviousVersion)

	return err

}

func (g gatekeeper) Name() string {
	return constants.GateKeeper
}

func NewGatekeeper() *gatekeeper {
	appPath, _ := absolutePathTo(constants.GateKeeper)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.GateKeeper)

	return &gatekeeper{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (g gatekeeper) install(ctx context.Context, env *environment.Env, appPath string) error {
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
