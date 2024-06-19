package appscenarios

import (
	"context"
	"path/filepath"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

type certManager struct{}

func (r certManager) Name() string {
	return "cert-manager"
}

var _ AppScenario = (*reloader)(nil)

func (r certManager) Install(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (r certManager) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
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

func (r certManager) Upgrade(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.install(ctx, env, appPath)
	if err != nil {
		return err
	}

	return err
}

func (r certManager) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the yaml for the namespace
	namespacePath := filepath.Join(appPath, "/cert-manager-namespace")
	err = env.ApplyYAML(ctx, namespacePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// create the priority class and resource quota
	priorityClassResourceQuotaPath := filepath.Join(appPath, "/priorityclass-resource-quota")
	err = env.ApplyYAML(ctx, priorityClassResourceQuotaPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the release
	releasePath := filepath.Join(appPath, "/release")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (r certManager) UpgradeRootCA(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.installRootCA(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (r certManager) InstallPreviousVersionRootCA(ctx context.Context, env *environment.Env) error {
	appPath, err := getkAppsUpgradePath(r.Name())
	if err != nil {
		return err
	}

	err = r.installRootCA(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (r certManager) InstallRootCA(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo(r.Name())
	if err != nil {
		return err
	}

	err = r.installRootCA(ctx, env, appPath)
	if err != nil {
		return err
	}

	return nil
}

func (r certManager) installRootCA(ctx context.Context, env *environment.Env, appPath string) error {
	// apply the yaml for the namespace
	rootCAPath := filepath.Join(appPath, "/root-ca")
	err := env.ApplyYAML(ctx, rootCAPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

func (r certManager) InstallTestCertificate(ctx context.Context, env *environment.Env) error {
	return r.installYaml(ctx, env, kommanderNamespace, "/cert-manager/test-cert")
}

func (r certManager) InstallStepCertificates(ctx context.Context, env *environment.Env) error {
	return r.installYaml(ctx, env, kommanderNamespace, "/cert-manager/acme-setup")
}

func (r certManager) CreateAcmeIssuer(ctx context.Context, env *environment.Env) error {
	return r.installYaml(ctx, env, kommanderNamespace, "/cert-manager/acme-clusterissuer")
}

func (r certManager) CreateAcmeCertificate(ctx context.Context, env *environment.Env) error {
	return r.installYaml(ctx, env, kommanderNamespace, "/cert-manager/acme-test-cert")
}

func (r certManager) installYaml(ctx context.Context, env *environment.Env, releaseNamespace string, directoryToInstall string) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	// apply the yaml for the namespace
	certificatePath := filepath.Join(testDataPath, directoryToInstall)
	err = env.ApplyYAML(ctx, certificatePath, map[string]string{
		"releaseNamespace": releaseNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}
