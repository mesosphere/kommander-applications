package appscenarios

import (
	"context"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"path/filepath"
)

type reloader struct{}

func (r reloader) Name() string {
	return "reloader"
}

var _ AppScenario = (*reloader)(nil)

func (r reloader) Install(ctx context.Context, env *environment.Env) error {
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

	return err
}
