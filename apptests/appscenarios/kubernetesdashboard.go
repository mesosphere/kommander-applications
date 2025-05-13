package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type kubernetesDashboard struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k kubernetesDashboard) Name() string {
	return constants.KubernetesDashboard
}

var _ scenarios.AppScenario = (*kubernetesDashboard)(nil)

func NewKubernetesDashboard() *kubernetesDashboard {
	appPath, _ := absolutePathTo(constants.KubernetesDashboard)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.KubernetesDashboard)
	return &kubernetesDashboard{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (k kubernetesDashboard) Install(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathCurrentVersion)
	return err
}

func (k kubernetesDashboard) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathPreviousVersion)

	return err
}

func (k kubernetesDashboard) InstallDependency(ctx context.Context, env *environment.Env, depAppName string) error {
	appPath, err := absolutePathTo(depAppName)
	if err != nil {
		return err
	}
	err = k.install(ctx, env, appPath)

	return err
}

func (k kubernetesDashboard) install(ctx context.Context, env *environment.Env, appPath string) error {
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
	// err = env.ApplyKustomizations(ctx, appPath, map[string]string{
	// 	"releaseNamespace":   kommanderNamespace,
	// 	"workspaceNamespace": kommanderNamespace,
	// })

	// apply the kustomization for the release
	releasePath := filepath.Join(appPath, "/helmrelease")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (k kubernetesDashboard) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
