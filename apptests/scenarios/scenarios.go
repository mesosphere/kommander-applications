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

// Has checks if the associated scenario for the given application exist.
func Has(application string) bool {
	_, ok := sc[application]
	return ok
}

// AbsolutePathTo returns the absolute path to the given application directory.
func AbsolutePathTo(application string) (string, error) {
	dir, err := filepath.Abs(filepath.Join("../../services/", application))
	if err != nil {
		return "", err
	}

	// os.ReadDir sorts the result alphabetically
	// so first entry in the slice always is application version directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	return entries[0].Name(), nil
}

// This is the List of all available scenarios.
var sc = List{
	"reloader": reloader{},
}
