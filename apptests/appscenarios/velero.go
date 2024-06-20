package appscenarios

import (
	"context"
	"os"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type velero struct{}

func (v velero) Name() string {
	return constants.Velero
}

var _ AppScenario = (*reloader)(nil)

func (v velero) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(v.Name())
	if err != nil {
		return err
	}

	err = v.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (v velero) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(v.Name())
	if err != nil {
		return err
	}

	err = v.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (v velero) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(v.Name())
	if err != nil {
		return err
	}

	err = v.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (v velero) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	preInstallPath := filepath.Join(appPath, "pre-install")
	err = env.ApplyYAML(ctx, preInstallPath, map[string]string{
		"releaseNamespace":         kommanderNamespace,
		"kubetoolsImageRepository": kubetoolsImageRepository,
		"kubetoolsImageTag":        kubetoolsImageTag,
	})
	if err != nil {
		return err
	}

	postInstallPath := filepath.Join(appPath, "post-install")
	err = env.ApplyYAML(ctx, postInstallPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	v4hooksPath := filepath.Join(appPath, "v4-hooks-adoption")
	// Check If the v4hooksPath path exists - it doesn't in app version 4.1.1
	// TODO: Remove this check once all previous versions have v4hooksPath
	_, err = os.Stat(v4hooksPath)
	if err == nil {
		err = env.ApplyYAML(ctx, v4hooksPath, map[string]string{
			"releaseNamespace":         kommanderNamespace,
			"kubetoolsImageRepository": kubetoolsImageRepository,
			"kubetoolsImageTag":        kubetoolsImageTag,
		})
		if err != nil {
			return err
		}
	}

	veleroPath := filepath.Join(appPath, "helmrelease")
	err = env.ApplyYAML(ctx, veleroPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	grafanaDashboardsPath := filepath.Join(appPath, "grafana-dashboards")
	err = env.ApplyKustomizations(ctx, grafanaDashboardsPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

// CreateNginxApp Creates a test Nginx app with a service and a deployment
func (v velero) CreateNginxApp(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	nginxPath := filepath.Join(testDataPath, "velero/nginx")
	err = env.ApplyYAMLWithoutSubstitutions(ctx, nginxPath)
	if err != nil {
		return err
	}

	return err
}

// Backup creates a backup with the given name
func (v velero) Backup(ctx context.Context, env *environment.Env, backupName string) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	nginxPath := filepath.Join(testDataPath, "velero/backup")
	err = env.ApplyYAML(ctx, nginxPath, map[string]string{
		"BACKUP_NAME": backupName,
	})
	if err != nil {
		return err
	}

	return err
}

// Restore restores a backup of the given name
func (v velero) Restore(ctx context.Context, env *environment.Env, backupName string, restoreName string) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	nginxPath := filepath.Join(testDataPath, "velero/restore")
	err = env.ApplyYAML(ctx, nginxPath, map[string]string{
		"BACKUP_NAME":  backupName,
		"RESTORE_NAME": restoreName,
	})
	if err != nil {
		return err
	}

	return err
}
