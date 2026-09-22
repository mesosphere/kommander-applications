package appscenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var upgradeKAppsRepoPath string

const catalogDirEnv = "NKP_CATALOG_DIR"

// catalogRoot returns the catalog root used for application manifests.
func catalogRoot() (string, error) {
	if dir := os.Getenv(catalogDirEnv); dir != "" {
		return filepath.Abs(dir)
	}

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not determine package root")
	}

	return filepath.Abs(filepath.Join(filepath.Dir(file), ".."))
}

// repositoryRoot returns the nkp-catalog-tests checkout root
// this is separate from NKP_CATALOG_DIR which points to the application directory
func repositoryRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not determine repository root")
	}

	return filepath.Abs(filepath.Join(filepath.Dir(file), ".."))
}

// absolutePathTo returns the absolute path to the given application directory
func absolutePathTo(application string) (string, error) {
	root, err := catalogRoot()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(root, "applications", application)

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
	root, err := repositoryRoot()
	if err != nil {
		return "", err
	}

	testDataDir := filepath.Join(root, "testdata")
	_, err = os.Stat(testDataDir)
	if err != nil {
		return "", fmt.Errorf("testdata directory not found: %w", err)
	}

	return testDataDir, nil
}

func getkAppsUpgradePath(application string) (string, error) {
	root, err := catalogRoot()
	if err != nil {
		return "", err
	}

	// Check that the app repo has been cloned
	upgradeRoot := filepath.Join(root, upgradeKAppsRepoPath)
	_, err = os.Stat(upgradeRoot)
	if err != nil {
		return "", fmt.Errorf("kommander-applications upgrade directory not found: %w", err)
	}

	// Get the absolute path to the application directory
	dir := filepath.Join(upgradeRoot, "applications", application)

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
