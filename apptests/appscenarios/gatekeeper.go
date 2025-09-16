package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	gatekeeperapi "github.com/open-policy-agent/frameworks/constraint/pkg/apis"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type gatekeeper struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

var _ scenarios.AppScenario = (*gatekeeper)(nil)

func setupGatekeeperSchema(env *environment.Env) error {
	scheme := flux.NewScheme()
	err := gatekeeperapi.AddToScheme(scheme)
	if err != nil {
		return err
	}
	c, err := genericCLient.New(env.K8sClient.Config(), genericCLient.Options{
		Scheme: scheme,
	})
	env.SetClient(c)
	return nil
}

func (g gatekeeper) Install(ctx context.Context, env *environment.Env) error {
	err := setupGatekeeperSchema(env)
	if err != nil {
		return err
	}
	err = g.install(ctx, env, g.appPathCurrentVersion)
	return err
}

func (g gatekeeper) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathPreviousVersion)

	return err

}

func (g gatekeeper) Name() string {
	return constants.GateKeeper
}

func NewGatekeeper() *gatekeeper {
	appPath, _ := absolutePathTo(constants.GateKeeper)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.GateKeeper)

	return &gatekeeper{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (g gatekeeper) install(ctx context.Context, env *environment.Env, appPath string) error {
	substMap := map[string]string{
		"releaseName":      "app-deployment-name",
		"releaseNamespace": kommanderNamespace,
	}
	// apply the gatekeeper HelmReleases
	err := env.ApplyKustomizations(ctx, filepath.Join(appPath, "/release"), substMap)
	if err != nil {
		return err
	}

	genericClient, err := genericCLient.New(env.K8sClient.Config(), genericCLient.Options{
		Scheme: flux.NewScheme(),
	})

	// ensure constrainttemplates CRD installed
	err = retry.OnError(wait.Backoff{
		Steps:    60,
		Duration: 5 * time.Second,
	}, func(err error) bool { return errors.IsNotFound(err) }, func() error {
		crdObj := apiextensionsv1.CustomResourceDefinition{}
		err = genericClient.Get(ctx, genericCLient.ObjectKey{
			Name: "constrainttemplates.templates.gatekeeper.sh",
		}, &crdObj)
		return err
	})
	if err != nil {
		return err
	}

	// apply gatekeeper constraints
	err = env.ApplyYAML(ctx, filepath.Join(appPath, "/constrainttemplates/enforce-sa-constrainttemplate.yaml"), substMap)
	if err != nil {
		return err
	}

	// ensure requiredserviceaccountname CRD installed
	err = retry.OnError(wait.Backoff{
		Steps:    60,
		Duration: 5 * time.Second,
	}, func(err error) bool { return errors.IsNotFound(err) }, func() error {
		crdObj := apiextensionsv1.CustomResourceDefinition{}
		err = genericClient.Get(ctx, genericCLient.ObjectKey{
			Name: "requiredserviceaccountname.constraints.gatekeeper.sh",
		}, &crdObj)
		return err
	})
	if err != nil {
		return err
	}

	err = env.ApplyYAML(ctx, filepath.Join(appPath, "/constraints/enforce-helmrelease-sa.yaml"), substMap)
	if err != nil {
		return err
	}
	err = env.ApplyYAML(ctx, filepath.Join(appPath, "/constraints/enforce-kustomization-sa.yaml"), substMap)

	return err
}

func (g gatekeeper) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
