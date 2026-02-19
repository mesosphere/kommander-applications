package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type grafanaLokiV3 struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (g grafanaLokiV3) Name() string {
	return constants.GrafanaLokiV3
}

var _ scenarios.AppScenario = (*grafanaLokiV3)(nil)

func NewGrafanaLokiV3() *grafanaLokiV3 {
	appPath, _ := absolutePathTo(constants.GrafanaLokiV3)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.GrafanaLokiV3)
	return &grafanaLokiV3{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (g grafanaLokiV3) Install(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathCurrentVersion)
	return err
}

func (g grafanaLokiV3) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := g.install(ctx, env, g.appPathPreviousVersion)
	return err
}

func (g grafanaLokiV3) install(ctx context.Context, env *environment.Env, appPath string) error {
	// Create the dkp-loki secret that would normally be provisioned by the
	// Object Bucket Claim (OBC) controller. In a Kind test cluster there is
	// no OBC controller, so we create a mock secret with dummy S3 credentials.
	if err := g.createOBCSecret(ctx, env); err != nil {
		return fmt.Errorf("creating OBC secret: %w", err)
	}

	// Deploy MinIO as a test S3 backend (required for distributed mode)
	if err := g.deployMinIO(ctx, env); err != nil {
		return fmt.Errorf("deploying MinIO: %w", err)
	}

	// Wait for MinIO to be ready and buckets to be created
	if err := g.waitForMinIOReady(ctx, env); err != nil {
		return fmt.Errorf("waiting for MinIO readiness: %w", err)
	}

	// Apply test overrides ConfigMap (MinIO S3, single ingester, etc.)
	if err := g.applyTestOverrides(ctx, env); err != nil {
		return fmt.Errorf("applying test overrides: %w", err)
	}

	// Apply only the helmrelease subdirectory which contains the raw
	// OCIRepository, HelmRelease, and ConfigMap (not the Flux Kustomization
	// wrappers which require a GitRepository that doesn't exist in test clusters).
	helmreleasePath := filepath.Join(appPath, "helmrelease")
	err := env.ApplyKustomizations(ctx, helmreleasePath, map[string]string{
		"appVersion":       "app-version-grafana-loki-v3",
		"releaseName":      "grafana-loki-v3",
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// Patch the HelmRelease to include the test overrides ConfigMap
	if err := g.patchHelmReleaseWithOverrides(ctx, env); err != nil {
		return fmt.Errorf("patching HelmRelease with overrides: %w", err)
	}

	return nil
}

func (g grafanaLokiV3) createOBCSecret(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dkp-loki",
			Namespace: kommanderNamespace,
		},
		StringData: map[string]string{
			"BUCKET_HOST":           "localhost",
			"BUCKET_PORT":           "80",
			"BUCKET_NAME":           "dkp-loki",
			"AWS_ACCESS_KEY_ID":     "test-access-key",
			"AWS_SECRET_ACCESS_KEY": "test-secret-key",
		},
	}

	return ctrlClient.IgnoreAlreadyExists(client.Create(ctx, secret))
}

func (g grafanaLokiV3) deployMinIO(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	minioPath := filepath.Join(testDataPath, "grafana-loki-v3", "minio.yaml")
	content, err := os.ReadFile(minioPath)
	if err != nil {
		return fmt.Errorf("failed to read MinIO manifest: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, content, map[string]string{
		"namespace": kommanderNamespace,
	})
}

func (g grafanaLokiV3) applyTestOverrides(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	overridePath := filepath.Join(testDataPath, "grafana-loki-v3", "overrides.yaml")
	content, err := os.ReadFile(overridePath)
	if err != nil {
		return fmt.Errorf("failed to read test overrides: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, content, map[string]string{
		"namespace": kommanderNamespace,
	})
}

func (g grafanaLokiV3) patchHelmReleaseWithOverrides(ctx context.Context, env *environment.Env) error {
	hr := &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.Name(),
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
		Name: "grafana-loki-v3-test-overrides",
	})
	err = genericClient.Update(ctx, hr)
	if err != nil {
		return fmt.Errorf("could not update the HelmRelease: %w", err)
	}

	return nil
}

func (g grafanaLokiV3) waitForMinIOReady(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	// Wait for MinIO pod to be ready
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	err = wait.PollUntilContextCancel(waitCtx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		podList := &corev1.PodList{}
		err := client.List(ctx, podList, &ctrlClient.ListOptions{
			Namespace:     kommanderNamespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{"app": "minio"}),
		})
		if err != nil {
			return false, nil
		}
		for _, pod := range podList.Items {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("MinIO pod not ready: %w", err)
	}

	// Wait for the bucket-creation job to complete
	waitCtx2, cancel2 := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel2()

	err = wait.PollUntilContextCancel(waitCtx2, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		job := &batchv1.Job{}
		err := client.Get(ctx, ctrlClient.ObjectKey{
			Namespace: kommanderNamespace,
			Name:      "minio-bucket-create",
		}, job)
		if err != nil {
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("MinIO bucket creation job not completed: %w", err)
	}

	return nil
}

func (g grafanaLokiV3) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
