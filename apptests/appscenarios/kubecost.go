package appscenarios

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type kubeCost struct{}

func (r kubeCost) Name() string {
	return constants.CentralizedKubecost
}

// OldName returns the name of the app pre upgrade.
// In 2.15.x we can drop this helper function and just use the Name() function again.
func (r kubeCost) OldName() string {
	return constants.KubeCost
}

var _ AppScenario = (*reloader)(nil)

func (r kubeCost) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (r kubeCost) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(r.OldName())
	if err != nil {
		return err
	}

	err = r.installOldKubecost(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (r kubeCost) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

// install installs the centralized-kubecost app
func (r kubeCost) install(ctx context.Context, env *environment.Env, appPath string) error {
	_, err := env.K8sClient.Clientset().CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kubecost"},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err = env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the prereqs
	prereqs := filepath.Join(appPath, "/pre-install")
	err = env.ApplyKustomizations(ctx, prereqs, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the helmrelease
	releasePath := filepath.Join(appPath, "/release")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (r kubeCost) installOldKubecost(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the helmrelease
	releasePath := filepath.Join(appPath, "/")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}
