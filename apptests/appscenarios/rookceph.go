package appscenarios

import (
	"context"
	"fmt"
	"path/filepath"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type rookCeph struct{}

func (r rookCeph) Name() string {
	return constants.RookCeph
}

var _ AppScenario = (*reloader)(nil)

func (r rookCeph) Install(ctx context.Context, env *environment.Env) error {
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

func (r rookCeph) CreateBuckets(ctx context.Context, env *environment.Env) error {
	appPath, err := absolutePathTo("rook-ceph-cluster")
	if err != nil {
		return err
	}

	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err = env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// Apply overrides configmap
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	cmPath := filepath.Join(testDataPath, "rook-ceph", "overrides")
	err = env.ApplyYAML(ctx, cmPath, nil)
	if err != nil {
		return err
	}

	// create the buckets
	objBucketClaimsPath := filepath.Join(appPath, "/objectbucketclaims")
	err = env.ApplyKustomizations(ctx, objBucketClaimsPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomizations for pre-install
	preInstallPath := filepath.Join(appPath, "/pre-install")
	err = env.ApplyYAML(ctx, preInstallPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the helmrelease
	releasePath := filepath.Join(appPath, "/helmrelease")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the helmrelease
	dashboardsPath := filepath.Join(appPath, "/grafana-dashboards")
	err = env.ApplyKustomizations(ctx, dashboardsPath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// Apply the patch to reference the overrides configmap
	err = r.applyRookCephOverrideCM(ctx, env, "rook-ceph-overrides")
	if err != nil {
		return err
	}

	return err
}

func (r rookCeph) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
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

func (r rookCeph) Upgrade(ctx context.Context, env *environment.Env) error {
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

func (r rookCeph) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults configmaps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// apply the kustomization for the helmrelease
	releasePath := filepath.Join(appPath, "/helmrelease")
	err = env.ApplyKustomizations(ctx, releasePath, map[string]string{
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return err
}

// applyRookCephOverrideCM applies the overrides configmap to the rook-ceph-cluster HelmRelease. This provides smaller
// sized buckets and single replicas for the test environment.
func (r rookCeph) applyRookCephOverrideCM(ctx context.Context, env *environment.Env, cmName string) error {
	hr := &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rook-ceph-cluster",
			Namespace: kommanderNamespace,
		},
	}

	genericClient, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("could not create the generic client: %w", err)
	}
	err = genericClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)

	if err != nil {
		return fmt.Errorf("could not get the HelmRelease: %w", err)
	}

	hr.Spec.ValuesFrom = append(hr.Spec.ValuesFrom, fluxhelmv2beta2.ValuesReference{
		Kind: "ConfigMap",
		Name: "rook-ceph-cluster-overrides",
	})
	err = genericClient.Update(ctx, hr)

	return nil
}

// CreateLoopbackDevicesKind creates loopback devices in the kind cluster as a workarround for the Rook Ceph installation.
func (r rookCeph) CreateLoopbackDevicesKind(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	// apply the yaml for the namespace
	scriptPath := filepath.Join(testDataPath, "rook-ceph/loop-devs")
	err = env.ApplyYAMLNoSubstitutions(ctx, scriptPath)
	if err != nil {
		return err
	}

	return nil
}

func (r rookCeph) ApplyPersistentVolumeCreator(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	// apply the yaml for the namespace
	scriptPath := filepath.Join(testDataPath, "rook-ceph/manifests")
	err = env.ApplyYAMLNoSubstitutions(ctx, scriptPath)
	if err != nil {
		return err
	}

	return nil
}
