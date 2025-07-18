package appscenarios

import (
	"fmt"
	"os"
	"path/filepath"
)

var upgradeKAppsRepoPath string

// absolutePathTo returns the absolute path to the given application directory.
func absolutePathTo(application string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// determining the execution path.
	var base string
	_, err = os.Stat(filepath.Join(wd, "applications"))
	if os.IsNotExist(err) {
		base = "../.."
	} else {
		base = ""
	}

	dir, err := filepath.Abs(filepath.Join(wd, base, "applications", application))
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
	dir, err := filepath.Abs(filepath.Join(wd, "../", upgradeKAppsRepoPath, "applications", application))
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
