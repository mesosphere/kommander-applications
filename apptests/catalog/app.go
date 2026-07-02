// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
	"sigs.k8s.io/yaml"
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
	return a.installWithDependencies(ctx, env, appPath, map[string]bool{})
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

func (a *App) installWithDependencies(
	ctx context.Context,
	env *environment.Env,
	appPath string,
	visiting map[string]bool,
) error {
	if visiting[a.Name()] {
		// Break potential cycles in dependency graph (A -> B -> A).
		return nil
	}
	visiting[a.Name()] = true
	defer delete(visiting, a.Name())

	requiredDeps, err := readRequiredDependencies(appPath)
	if err != nil {
		return fmt.Errorf("reading required dependencies for %s: %w", a.Name(), err)
	}

	skip := dependencySkipSet()
	for _, depName := range requiredDeps {
		if depName == a.Name() {
			continue
		}
		if skip[depName] {
			// The caller (e.g. an e2e suite) has pre-provisioned this dependency
			// out-of-band -- typically a lightweight CRDs-only shim -- to avoid
			// installing a heavy chart (e.g. istio-helm -> kube-prometheus-stack)
			// on a resource-constrained Kind cluster. Honor that and move on.
			continue
		}

		dep := &App{name: depName}
		depPath, err := absolutePathTo(depName, "")
		if err != nil {
			return fmt.Errorf("resolving dependency %s for %s: %w", depName, a.Name(), err)
		}
		if err := dep.installWithDependencies(ctx, env, depPath, visiting); err != nil {
			return err
		}
		if err := waitForDefaultReconcilerReady(depName, 10*time.Minute); err != nil {
			return fmt.Errorf("waiting for dependency %s to become ready: %w", depName, err)
		}
	}

	return a.install(ctx, env, appPath)
}

// InstallPreviousVersion installs the second-to-latest version for upgrade testing.
func (a *App) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getPrevVersionPath(a.Name())
	if err != nil {
		return err
	}
	return a.installWithDependencies(ctx, env, appPath, map[string]bool{})
}

// Upgrade applies the latest version over a previously installed version.
func (a *App) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(a.Name(), "")
	if err != nil {
		return err
	}
	return a.installWithDependencies(ctx, env, appPath, map[string]bool{})
}

// HasPreviousVersion reports whether the app has a version directory that
// sorts before the configured version, making an upgrade test meaningful.
func (a *App) HasPreviousVersion() bool {
	dir, err := applicationDir(a.name)
	if err != nil {
		return false
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*"))
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

	matches, err := filepath.Glob(filepath.Join(dir, "*"))
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

	matches, err := filepath.Glob(filepath.Join(dir, "*"))
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

	searchRoots, err := catalogSearchRoots(wd)
	if err != nil {
		return "", err
	}

	for _, root := range searchRoots {
		candidate := filepath.Join(root, "applications", application)
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return filepath.Abs(candidate)
		}
	}

	return "", fmt.Errorf("application directory for %s not found in search roots: %v", application, searchRoots)
}

// catalogSearchRoots returns repo roots that contain catalog applications.
//
// Resolution order:
//  1. APPTESTS_CATALOG_PATHS (comma-separated absolute or relative paths)
//  2. Current repo root (derived from working dir; supports running from repo root
//     or apptests subdir)
func catalogSearchRoots(wd string) ([]string, error) {
	roots := make([]string, 0, 4)
	seen := map[string]bool{}

	addRoot := func(path string) {
		if path == "" {
			return
		}
		abs, err := filepath.Abs(path)
		if err != nil || seen[abs] {
			return
		}
		if st, err := os.Stat(filepath.Join(abs, "applications")); err == nil && st.IsDir() {
			seen[abs] = true
			roots = append(roots, abs)
		}
	}

	if configured := strings.TrimSpace(os.Getenv("APPTESTS_CATALOG_PATHS")); configured != "" {
		for _, entry := range strings.Split(configured, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			addRoot(entry)
		}
	}

	// Current repo root fallback (works when cwd is either repo root or apptests dir).
	addRoot(wd)
	primaryRoot := filepath.Join(wd, "../..")
	addRoot(primaryRoot)

	// Sibling repo fallback (common local-dev layout):
	// <workspace>/nkp-ai-applications-catalog and <workspace>/kommander-applications.
	workspaceRoot := filepath.Clean(filepath.Join(primaryRoot, ".."))
	if entries, err := os.ReadDir(workspaceRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Avoid hidden/system dirs and skip current root already added.
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			addRoot(filepath.Join(workspaceRoot, name))
		}
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("no catalog roots found; set APPTESTS_CATALOG_PATHS to one or more repo roots containing applications/")
	}
	return roots, nil
}

type appMetadata struct {
	RequiredDependencies []string `yaml:"requiredDependencies"`
}

// dependencySkipSet returns the set of requiredDependencies that the auto-
// installer should NOT install itself, sourced from the APPTESTS_SKIP_DEPENDENCIES
// env var (comma-separated app names). An e2e suite sets this when it has already
// provisioned the dependency out-of-band (e.g. istio/cert-manager CRD shims).
func dependencySkipSet() map[string]bool {
	skip := map[string]bool{}
	for _, name := range strings.Split(os.Getenv("APPTESTS_SKIP_DEPENDENCIES"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			skip[name] = true
		}
	}
	return skip
}

func readRequiredDependencies(appPath string) ([]string, error) {
	metadataPath := filepath.Join(appPath, "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var md appMetadata
	if err := yaml.Unmarshal(content, &md); err != nil {
		return nil, err
	}
	return md.RequiredDependencies, nil
}
