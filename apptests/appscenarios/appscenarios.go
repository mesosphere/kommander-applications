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
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

var upgradeKAppsRepoPath string

// AppScenario defines the behavior and name of an application test scenario
type AppScenario interface {
	Name() string                                    // scenario name
	Install(context.Context, *environment.Env) error // logic implemented by a scenario
	InstallPreviousVersion(ctx context.Context, env *environment.Env) error
	Upgrade(ctx context.Context, env *environment.Env) error
}

// absolutePathTo returns the absolute path to the given application directory.
func absolutePathTo(application string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// determining the execution path.
	var base string
	_, err = os.Stat(filepath.Join(wd, "services"))
	if os.IsNotExist(err) {
		base = "../.."
	} else {
		base = ""
	}

	dir, err := filepath.Abs(filepath.Join(wd, base, "services", application))
	if err != nil {
		return "", err
	}

	// filepath.Glob returns a sorted slice of matching paths
	matches, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf(
			"no application directory found for %s in the given path:%s",
			application, dir)
	}

	return matches[0], nil

}

// getTestDataDir gets the directory path for test data
func getTestDataDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	_, err = os.Stat(filepath.Join(wd, "../testdata"))
	if err != nil {
		return "", fmt.Errorf("testdata directory not found: %w", err)
	}

	return filepath.Abs(filepath.Join(wd, "../testdata"))
}

func getkAppsUpgradePath(application string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check that the app repo has been cloned
	_, err = os.Stat(filepath.Join(wd, "../", upgradeKAppsRepoPath))
	if err != nil {
		return "", fmt.Errorf("kommander-applications upgrade directory not found: %w", err)
	}

	// Get the absolute path to the application directory
	dir, err := filepath.Abs(filepath.Join(wd, "../", upgradeKAppsRepoPath, "services", application))
	if err != nil {
		return "", err
	}

	// filepath.Glob returns a sorted slice of matching paths
	matches, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf(
			"no application directory found for %s in the given path:%s",
			application, dir)
	}

	return matches[0], nil
}
