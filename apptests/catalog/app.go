// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

const (
	DefaultNamespace = "default"
	PollInterval     = 2 * time.Second
)

// App is a generic AppScenario implementation that works for any catalog
// application using the standard helmrelease kustomization layout:
// applications/<name>/<version>/helmrelease/.
type App struct {
	name                string
	appVersionToInstall string
}

var _ scenarios.AppScenario = (*App)(nil)

// NewAppScenario creates a generic App scenario for the named application.
// When appVersionToInstall is empty, the latest (lexicographically last)
// version directory is used.
func NewAppScenario(name, appVersionToInstall string) scenarios.AppScenario {
	return &App{
		name:                name,
		appVersionToInstall: appVersionToInstall,
	}
}

// Name returns the application directory name.
func (a *App) Name() string {
	return a.name
}

// Install applies the helmrelease kustomization for the configured version.
func (a *App) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(a.Name(), a.appVersionToInstall)
	if err != nil {
		return err
	}
	return a.install(ctx, env, appPath)
}

func (a *App) install(ctx context.Context, env *environment.Env, appPath string) error {
	helmreleasePath := filepath.Join(appPath, "helmrelease")
	version := filepath.Base(appPath)
	return env.ApplyKustomizations(ctx, helmreleasePath, map[string]string{
		"releaseNamespace":   DefaultNamespace,
		"releaseName":        a.Name(),
		"workspaceNamespace": DefaultNamespace,
		"appVersion":         version,
	})
}

// InstallPreviousVersion installs the second-to-latest version for upgrade testing.
func (a *App) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getPrevVersionPath(a.Name())
	if err != nil {
		return err
	}
	return a.install(ctx, env, appPath)
}

// Upgrade applies the latest version over a previously installed version.
func (a *App) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(a.Name(), "")
	if err != nil {
		return err
	}
	return a.install(ctx, env, appPath)
}

// versionDirs returns all subdirectories of dir, sorted lexicographically.
// os.ReadDir guarantees lexicographic ordering, so no explicit sort is needed.
// Only directories are returned; non-directory entries such as .catalog-source.yaml
// or README.md are skipped to avoid being mistaken for version directories.
func versionDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(dir, e.Name()))
		}
	}
	return dirs, nil
}

// HasPreviousVersion reports whether the app has a version directory that
// sorts before the configured version, making an upgrade test meaningful.
func (a *App) HasPreviousVersion() bool {
	dir, err := applicationDir(a.name)
	if err != nil {
		return false
	}
	matches, err := versionDirs(dir)
	if err != nil {
		return false
	}
	if a.appVersionToInstall == "" {
		return len(matches) >= 2
	}
	for _, m := range matches {
		if filepath.Base(m) < a.appVersionToInstall {
			return true
		}
	}
	return false
}

// absolutePathTo returns the absolute path to the given application version directory.
// When appVersion is empty it returns the latest (lexicographically last) version.
func absolutePathTo(application, appVersion string) (string, error) {
	dir, err := applicationDir(application)
	if err != nil {
		return "", err
	}

	if appVersion != "" {
		pathToApp := filepath.Join(dir, appVersion)
		if _, err := os.Stat(pathToApp); err != nil {
			return "", fmt.Errorf("no application directory found for app: %s of version: %s", application, appVersion)
		}
		return pathToApp, nil
	}

	matches, err := versionDirs(dir)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no application directory found for %s in the given path: %s", application, dir)
	}

	return matches[len(matches)-1], nil
}

// getPrevVersionPath returns the second-to-latest version directory.
func getPrevVersionPath(application string) (string, error) {
	dir, err := applicationDir(application)
	if err != nil {
		return "", err
	}

	matches, err := versionDirs(dir)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no application directory found for %s in the given path: %s", application, dir)
	}
	if len(matches) < 2 {
		return "", fmt.Errorf("no old version found for the application: %s", application)
	}

	return matches[len(matches)-2], nil
}

func applicationDir(application string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	var base string
	if _, err := os.Stat(filepath.Join(wd, "applications")); os.IsNotExist(err) {
		base = "../.."
	}

	return filepath.Abs(filepath.Join(wd, base, "applications", application))
}
