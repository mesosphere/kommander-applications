package appscenarios

import (
	"context"
	"path/filepath"
	"time"

	"github.com/mesosphere/kommander-applications/apptests/environment"
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
