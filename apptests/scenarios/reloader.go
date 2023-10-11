package scenarios

import (
	"context"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type reloader struct{}

var _ Scenario = (*reloader)(nil)

func (r reloader) Execute(ctx context.Context, env *environment.Env) error {
	appPath, err := AbsolutePathTo("reloader")
	if err != nil {
		return err
	}

	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err = env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace": "kommander",
	})
	if err != nil {
		return err
	}
	// apply the rest of kustomizations
	err = env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace": "kommander",
	})
	if err != nil {
		return err
	}

	return nil
}
