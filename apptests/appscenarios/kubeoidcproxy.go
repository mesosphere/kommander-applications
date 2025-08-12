package appscenarios

import (
	"context"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type kubeOIDCProxy struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k kubeOIDCProxy) Name() string {
	return constants.KubeOIDCProxy
}

var _ scenarios.AppScenario = (*kubeOIDCProxy)(nil)

func NewKubeOIDCProxy() *kubeOIDCProxy {
	appPath, _ := absolutePathTo(constants.KubeOIDCProxy)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.KubeOIDCProxy)
	return &kubeOIDCProxy{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (k kubeOIDCProxy) Install(ctx context.Context, env *environment.Env) error {
	return k.install(ctx, env, k.appPathCurrentVersion)
}

func (k kubeOIDCProxy) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	return k.install(ctx, env, k.appPathPreviousVersion)
}

func (k kubeOIDCProxy) Upgrade(ctx context.Context, env *environment.Env) error {
	return k.install(ctx, env, k.appPathCurrentVersion)
}

func (k kubeOIDCProxy) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}
	// apply the rest of kustomizations
	return env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
}
