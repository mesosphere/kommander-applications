// Package appscenarios provides a set of application test scenarios that can be executed
// in a Kubernetes environment. The package defines an AppScenario interface that specifies the
// behavior and name of each scenario, and a List type that implements methods to execute, get,
// and check scenarios.
//
// The package currently supports one scenario for the reloader application, but more scenarios can be
// added by implementing the AppScenario interface and registering them in the scenariosList variable.
package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

// AppScenario defines the behavior and name of an application test scenario
type AppScenario interface {
	Execute(context.Context, *environment.Env) error // logic implemented by a scenario
	Name() string                                    // scenario name
}

type List map[string]AppScenario

// Execute runs all the scenarios in the list and returns the first error encountered, if any.
func (s List) Execute(ctx context.Context, env *environment.Env) error {
	for _, sc := range s {
		if err := sc.Execute(ctx, env); err != nil {
			return err
		}
	}
	return nil
}

// Get returns the associated scenario for the given application name, or nil if it does not exist.
func Get(application string) AppScenario {
	s, ok := scenariosList[application]
	if !ok {
		return nil
	}
	return s
}

// Has checks if the associated scenario for the given application exist.
func Has(application string) bool {
	_, ok := scenariosList[application]
	return ok
}

// AbsolutePathTo returns the absolute path to the given application directory.
func AbsolutePathTo(application string) (string, error) {
	dir, err := filepath.Abs(filepath.Join("../../services/", application))
	if err != nil {
		return "", err
	}

	// filepath.Glob returns a sorted slice of matching paths
	matches, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no application directory found for %s", application)
	}

	return matches[0], nil
}

// This is the ScenarioList of all available scenarios.
var scenariosList = List{
	"reloader": reloader{},
}
