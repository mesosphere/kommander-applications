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

// MultiClusterAppScenario defines the behavior for multi-cluster application test scenarios.
// It extends AppScenario with support for testing applications across management and attached clusters.
type MultiClusterAppScenario interface {
	Name() string
	Install(ctx context.Context, env *environment.MultiClusterEnv) error
	InstallPreviousVersion(ctx context.Context, env *environment.MultiClusterEnv) error
	Upgrade(ctx context.Context, env *environment.MultiClusterEnv) error
}
