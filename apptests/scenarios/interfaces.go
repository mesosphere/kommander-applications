// Package scenarios defines an AppScenario interface that specifies the
// behavior and name of each scenario
package scenarios

import (
	"context"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

// AppScenario defines the behavior and name of an application test scenario
type AppScenario interface {
	Name() string                                    // scenario name
	Install(context.Context, *environment.Env) error // logic implemented by a scenario
	InstallPreviousVersion(ctx context.Context, env *environment.Env) error
	Upgrade(ctx context.Context, env *environment.Env) error
}
