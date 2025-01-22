package appscenarios

import (
	"context"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type kubeCost struct{}

func (r kubeCost) Name() string {
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
	appPath, err := getkAppsUpgradePath(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
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

func (r kubeCost) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// Kubecost has been restructured in 2.14.x. For upgrades to work, we need to handle both versions gracefully.
	helmReleasePath := filepath.Join(appPath, "/release")
	if _, err = os.Stat(helmReleasePath); err == nil {
		// kubecost installation requires that a secret named "tls-root-ca" exists in the installation namespace. It's fine if the secret is empty.
		if err = r.satisfyKubecostPrerequisites(ctx, env); err != nil {
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
		err = env.ApplyKustomizations(ctx, helmReleasePath, map[string]string{
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return err
		}
		return nil
	}

	// apply the helmrelease which is at the "/" path up to 2.13.x
	return env.ApplyKustomizations(ctx, filepath.Join(appPath, "/"), map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
}

func (r kubeCost) satisfyKubecostPrerequisites(ctx context.Context, env *environment.Env) error {
	genericClient, err := genericCLient.New(env.K8sClient.Config(), genericCLient.Options{})
	if err != nil {
		return err
	}
	return genericClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-root-ca",
			Namespace: kommanderNamespace,
		},
	})
}
