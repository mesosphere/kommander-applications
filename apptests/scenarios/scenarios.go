package scenarios

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type Scenario interface {
	Execute(context.Context, *environment.Env) error
}

type List map[string]Scenario

// Execute runs all the scenarios in the list and returns the first error encountered, if any.
func (s List) Execute(ctx context.Context, env *environment.Env) error {
	for _, sc := range s {
		if err := sc.Execute(ctx, env); err != nil {
			return err
		}
	}
	return nil
}

// Get returns the associated scenario for the given application name.
func Get(application string) Scenario {
	s, ok := sc[application]
	if !ok {
		return s
	}

	return nil
}

// AbsolutePathTo returns the absolute path to the given application directory.
func AbsolutePathTo(application string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, "../../services/", application), nil
}

// AbsolutePathToBase returns the absolute path to common/base directory.
func AbsolutePathToBase() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, "../../common/base"), nil
}

// This is the List of all available scenarios.
var sc = List{
	"reloader": reloader{},
}
