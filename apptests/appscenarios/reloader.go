package appscenarios

import (
	"context"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"path/filepath"
)

type reloader struct{}

func (r reloader) Name() string {
	return "reloader"
}

var _ AppScenario = (*reloader)(nil)

var nginxCMName = "nginx-config"

func (r reloader) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)

	return err
}

func (r reloader) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomizations := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomizations, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}
	// apply the rest of kustomizations
	err = env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (r reloader) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (r reloader) ApplyNginxConfigmap(ctx context.Context, env *environment.Env, nginxCMFilename string) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	nginxCMYamlPath := filepath.Join(testDataPath, "reloader", nginxCMFilename)
	err = env.ApplyYAML(ctx, nginxCMYamlPath, map[string]string{
		"namespace": kommanderNamespace,
		"cmName":    nginxCMName,
	})

	if err != nil {
		return err
	}

	return nil
}

func (r reloader) ApplyNginxDeployment(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	nginxDeploymentYamlPath := filepath.Join(testDataPath, "reloader/nginx.yaml")
	err = env.ApplyYAML(ctx, nginxDeploymentYamlPath, map[string]string{
		"namespace": kommanderNamespace,
		"cmName":    nginxCMName,
	})

	if err != nil {
		return err
	}

	return nil
}
