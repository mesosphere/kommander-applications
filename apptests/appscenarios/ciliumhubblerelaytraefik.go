package appscenarios

import (
	"context"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type ciliumHubbleRelayTraefik struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (c ciliumHubbleRelayTraefik) Name() string {
	return constants.CiliumHubbleRelayTraefik
}

var _ AppScenario = (*ciliumHubbleRelayTraefik)(nil)

func NewCiliumHubbleRelayTraefik() *ciliumHubbleRelayTraefik {
	appPath, _ := absolutePathTo(constants.CiliumHubbleRelayTraefik)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.CiliumHubbleRelayTraefik)
	return &ciliumHubbleRelayTraefik{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (c ciliumHubbleRelayTraefik) Install(ctx context.Context, env *environment.Env) error {
	err := c.install(ctx, env, c.appPathCurrentVersion)
	return err
}

func (c ciliumHubbleRelayTraefik) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := c.install(ctx, env, c.appPathPreviousVersion)

	return err
}

func (c ciliumHubbleRelayTraefik) InstallDependency(ctx context.Context, env *environment.Env, depAppName string) error {
	appPath, err := absolutePathTo(depAppName)
	if err != nil {
		return err
	}
	err = c.install(ctx, env, appPath)

	return err
}

func (c ciliumHubbleRelayTraefik) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}
	// apply the rest of kustomizations
	err = env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}
