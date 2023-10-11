package scenarios

import (
	"context"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type reloader struct{}

var _ Scenario = (*reloader)(nil)

func (r reloader) Execute(ctx context.Context, env *environment.Env) error {

	return nil
}
