package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type kubeOidcProxy struct{}

func (k kubeOidcProxy) Name() string {
	return "kube-oidc-proxy"
}

var _ scenarios.AppScenario = (*kubeOidcProxy)(nil)

func (k kubeOidcProxy) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(k.Name())
	if err != nil {
		return fmt.Errorf("getting app path: %w", err)
	}
	return k.install(ctx, env, appPath)
}

func (k kubeOidcProxy) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(k.Name())
	if err != nil {
		return fmt.Errorf("getting previous version path: %w", err)
	}
	return k.install(ctx, env, appPath)
}

func (k kubeOidcProxy) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(k.Name())
	if err != nil {
		return fmt.Errorf("getting app path: %w", err)
	}
	return k.install(ctx, env, appPath)
}

func (k kubeOidcProxy) install(ctx context.Context, env *environment.Env, appPath string) error {
	// Apply namespace
	namespacePath := filepath.Join(appPath, "namespace")
	if err := env.ApplyKustomizations(ctx, namespacePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	}); err != nil {
		return fmt.Errorf("applying namespace: %w", err)
	}

	releasePath := filepath.Join(appPath, "/release")
	if err := env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	}); err != nil {
		return fmt.Errorf("applying release: %w", err)
	}

	return nil
}
