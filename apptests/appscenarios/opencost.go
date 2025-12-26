package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

// openCost implements the AppScenario interface for single-cluster OpenCost deployment.
type openCost struct{}

var _ scenarios.AppScenario = (*openCost)(nil)

func (o openCost) Name() string {
	return constants.OpenCost
}

func (o openCost) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(o.Name())
	if err != nil {
		return err
	}

	return o.install(ctx, env, appPath)
}

func (o openCost) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(o.Name())
	if err != nil {
		return err
	}

	return o.install(ctx, env, appPath)
}

func (o openCost) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(o.Name())
	if err != nil {
		return err
	}

	return o.install(ctx, env, appPath)
}

func (o openCost) install(ctx context.Context, env *environment.Env, appPath string) error {
	fmt.Println("Installing OpenCost from path", appPath)

	// Check for pre-install directory and apply if it exists
	preInstallPath := filepath.Join(appPath, "pre-install")
	if _, err := os.Stat(preInstallPath); err == nil {
		err := env.ApplyKustomizations(ctx, preInstallPath, map[string]string{
			"releaseName":      "opencost",
			"appVersion":       "opencost-version",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return fmt.Errorf("failed to apply pre-install: %w", err)
		}
	}

	// Apply the release kustomization
	releasePath := filepath.Join(appPath, "release")
	if _, err := os.Stat(releasePath); err == nil {
		return env.ApplyKustomizations(ctx, releasePath, map[string]string{
			"releaseName":      "opencost",
			"appVersion":       "opencost-version",
			"releaseNamespace": kommanderNamespace,
		})
	}

	// Fallback to root path if release directory doesn't exist
	return env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseName":      "opencost",
		"appVersion":       "opencost-version",
		"releaseNamespace": kommanderNamespace,
	})
}

// centralizedOpenCost implements the AppScenario interface for centralized OpenCost deployment
// on the management cluster.
type centralizedOpenCost struct{}

var _ scenarios.AppScenario = (*centralizedOpenCost)(nil)

func (c centralizedOpenCost) Name() string {
	return constants.CentralizedOpenCost
}

func (c centralizedOpenCost) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(c.Name())
	if err != nil {
		return err
	}

	return c.install(ctx, env, appPath)
}

func (c centralizedOpenCost) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(c.Name())
	if err != nil {
		return err
	}

	return c.install(ctx, env, appPath)
}

func (c centralizedOpenCost) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(c.Name())
	if err != nil {
		return err
	}

	return c.install(ctx, env, appPath)
}

func (c centralizedOpenCost) install(ctx context.Context, env *environment.Env, appPath string) error {
	fmt.Println("Installing Centralized OpenCost from path", appPath)

	// Apply the release kustomization
	releasePath := filepath.Join(appPath, "release")
	if _, err := os.Stat(releasePath); err == nil {
		return env.ApplyKustomizations(ctx, releasePath, map[string]string{
			"releaseName":        "centralized-opencost",
			"appVersion":         "centralized-opencost-version",
			"releaseNamespace":   kommanderNamespace,
			"workspaceNamespace": kommanderNamespace,
		})
	}

	// Fallback to root path if release directory doesn't exist
	return env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseName":        "centralized-opencost",
		"appVersion":         "centralized-opencost-version",
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
}

// multiClusterOpenCost implements the MultiClusterAppScenario interface for deploying
// OpenCost in a multi-cluster environment with centralized-opencost on the management cluster
// and opencost on the workload cluster.
type multiClusterOpenCost struct {
	openCost            openCost
	centralizedOpenCost centralizedOpenCost
}

var _ scenarios.MultiClusterAppScenario = (*multiClusterOpenCost)(nil)

func (m multiClusterOpenCost) Name() string {
	return "multicluster-opencost"
}

// Install installs centralized-opencost on the management cluster and opencost on the workload cluster.
func (m multiClusterOpenCost) Install(ctx context.Context, env *environment.MultiClusterEnv) error {
	// Install centralized-opencost on the management cluster
	fmt.Println("Installing centralized-opencost on management cluster...")
	if err := m.centralizedOpenCost.Install(ctx, env.ManagementEnv); err != nil {
		return fmt.Errorf("failed to install centralized-opencost on management cluster: %w", err)
	}

	// Install opencost on the workload cluster
	fmt.Println("Installing opencost on workload cluster...")
	if err := m.openCost.Install(ctx, env.WorkloadEnv); err != nil {
		return fmt.Errorf("failed to install opencost on workload cluster: %w", err)
	}

	return nil
}

// InstallPreviousVersion installs the previous version of opencost components for upgrade testing.
func (m multiClusterOpenCost) InstallPreviousVersion(ctx context.Context, env *environment.MultiClusterEnv) error {
	// Install previous version of centralized-opencost on the management cluster
	fmt.Println("Installing previous version of centralized-opencost on management cluster...")
	if err := m.centralizedOpenCost.InstallPreviousVersion(ctx, env.ManagementEnv); err != nil {
		return fmt.Errorf("failed to install previous centralized-opencost on management cluster: %w", err)
	}

	// Install previous version of opencost on the workload cluster
	fmt.Println("Installing previous version of opencost on workload cluster...")
	if err := m.openCost.InstallPreviousVersion(ctx, env.WorkloadEnv); err != nil {
		return fmt.Errorf("failed to install previous opencost on workload cluster: %w", err)
	}

	return nil
}

// Upgrade upgrades both opencost components to the current version.
func (m multiClusterOpenCost) Upgrade(ctx context.Context, env *environment.MultiClusterEnv) error {
	// Upgrade centralized-opencost on the management cluster
	fmt.Println("Upgrading centralized-opencost on management cluster...")
	if err := m.centralizedOpenCost.Upgrade(ctx, env.ManagementEnv); err != nil {
		return fmt.Errorf("failed to upgrade centralized-opencost on management cluster: %w", err)
	}

	// Upgrade opencost on the workload cluster
	fmt.Println("Upgrading opencost on workload cluster...")
	if err := m.openCost.Upgrade(ctx, env.WorkloadEnv); err != nil {
		return fmt.Errorf("failed to upgrade opencost on workload cluster: %w", err)
	}

	return nil
}

// ManagementComponentName returns the name of the component installed on the management cluster.
func (m multiClusterOpenCost) ManagementComponentName() string {
	return m.centralizedOpenCost.Name()
}

// WorkloadComponentName returns the name of the component installed on the workload cluster.
func (m multiClusterOpenCost) WorkloadComponentName() string {
	return m.openCost.Name()
}

