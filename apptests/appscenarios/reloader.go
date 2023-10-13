package appscenarios

import (
	"context"
	"path/filepath"
	"time"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"
)

type reloader struct{}

func (r reloader) Name() string {
	return "reloader"
}

var _ AppScenario = (*reloader)(nil)

const (
	pollInterval           = 2 * time.Second
	kommanderNamespace     = "kommander"
	kommanderFluxNamespace = "kommander-flux"
)

func (r reloader) Execute(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err = env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
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

	client, err := genericCLient.New(env.K8sClient.Config(), genericCLient.Options{Scheme: flux.NewScheme()})
	if err != nil {
		return err
	}

	hr := &fluxhelmv2beta1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta1.HelmReleaseKind,
			APIVersion: fluxhelmv2beta1.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Name(),
			Namespace: kommanderNamespace,
		},
	}

	err = wait.PollUntilContextCancel(ctx, pollInterval, true, func(ctx context.Context) (done bool, err error) {
		err = client.Get(ctx, genericCLient.ObjectKeyFromObject(hr), hr)
		if err != nil {
			return false, err
		}

		for _, cond := range hr.Status.Conditions {
			if cond.Status == metav1.ConditionTrue &&
				cond.Type == fluxhelmv2beta1.ReleasedCondition {
				return true, nil
			}
		}
		return false, nil
	})

	return err
}
