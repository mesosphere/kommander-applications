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

type ciliumHubbleRelayTraefik struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (c ciliumHubbleRelayTraefik) Name() string {
	return constants.CiliumHubbleRelayTraefik
}

var _ scenarios.AppScenario = (*ciliumHubbleRelayTraefik)(nil)

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

func (c ciliumHubbleRelayTraefik) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"appVersion":         "app-version" + filepath.Base(appPath),
			"releaseNamespace":   kommanderNamespace,
			"workspaceNamespace": kommanderNamespace,
		})
		if err != nil {
			return err
		}
	}
	// apply the rest of kustomizations
	err := env.ApplyKustomizations(ctx, appPath, map[string]string{
		"appVersion":         "app-version" + filepath.Base(appPath),
		"releaseName":        "app-deployment-name" + filepath.Base(appPath),
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (c ciliumHubbleRelayTraefik) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
