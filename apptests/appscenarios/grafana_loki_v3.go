package appscenarios

import (
	"context"
	"fmt"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type grafanaLokiV3 struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (g grafanaLokiV3) Name() string {
	return constants.GrafanaLokiV3
}

var _ scenarios.AppScenario = (*grafanaLokiV3)(nil)

func NewGrafanaLokiV3() *grafanaLokiV3 {
	appPath, _ := absolutePathTo(constants.GrafanaLokiV3)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.GrafanaLokiV3)
	return &grafanaLokiV3{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (g grafanaLokiV3) Install(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathCurrentVersion)
	return err
}

func (g grafanaLokiV3) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathPreviousVersion)
	return err
}

func (g grafanaLokiV3) install(ctx context.Context, env *environment.Env, appPath string) error {
	// Apply the loki v3 kustomizations
	err := env.ApplyKustomizations(ctx, appPath, map[string]string{
		"appVersion":       "app-version-grafana-loki-v3",
		"releaseName":      "grafana-loki-v3",
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}

func (g grafanaLokiV3) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
