package appscenarios

import (
	"context"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

// openCost implements the AppScenario interface for single-cluster OpenCost deployment.
type openCost struct{}

var _ scenarios.MultiClusterAppScenario = (*openCost)(nil)

func (o openCost) Name() string {
	return constants.OpenCost
}

func (o openCost) Install(ctx context.Context, env *environment.MultiClusterEnv) error {
	//TODO implement me
	panic("implement me")
}

func (o openCost) InstallPreviousVersion(ctx context.Context, env *environment.MultiClusterEnv) error {
	//TODO implement me
	panic("implement me")
}

func (o openCost) Upgrade(ctx context.Context, env *environment.MultiClusterEnv) error {
	//TODO implement me
	panic("implement me")
}
