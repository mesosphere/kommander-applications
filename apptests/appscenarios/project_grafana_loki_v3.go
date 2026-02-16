package appscenarios

import (
	"context"
	"fmt"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type projectGrafanaLokiV3 struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (p projectGrafanaLokiV3) Name() string {
	return constants.ProjectGrafanaLokiV3
}

var _ scenarios.AppScenario = (*projectGrafanaLokiV3)(nil)

func NewProjectGrafanaLokiV3() *projectGrafanaLokiV3 {
	appPath, _ := absolutePathTo(constants.ProjectGrafanaLokiV3)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.ProjectGrafanaLokiV3)
	return &projectGrafanaLokiV3{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (p projectGrafanaLokiV3) Install(ctx context.Context, env *environment.Env) error {
	err := p.install(ctx, env, p.appPathCurrentVersion)
	return err
}

func (p projectGrafanaLokiV3) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := p.install(ctx, env, p.appPathPreviousVersion)
	return err
}

func (p projectGrafanaLokiV3) install(ctx context.Context, env *environment.Env, appPath string) error {
	// Apply the project loki v3 kustomizations
	// Project-level apps typically deploy to a project namespace
	err := env.ApplyKustomizations(ctx, appPath, map[string]string{
		"appVersion":         "app-version-project-grafana-loki-v3",
		"releaseName":        "project-grafana-loki-v3",
		"releaseNamespace":   kommanderNamespace, // Project apps still deploy resources to workspace namespace
		"workspaceNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}

func (p projectGrafanaLokiV3) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
