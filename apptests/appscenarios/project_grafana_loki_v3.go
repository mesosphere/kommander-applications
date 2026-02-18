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

type projectGrafanaLokiV3 struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (p projectGrafanaLokiV3) Name() string {
	return constants.ProjectGrafanaLokiV3
}

var _ scenarios.AppScenario = (*projectGrafanaLokiV3)(nil)

func NewProjectGrafanaLokiV3() *projectGrafanaLokiV3 {
	appPath, _ := absolutePathTo(constants.ProjectGrafanaLokiV3)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.ProjectGrafanaLokiV3)
	return &projectGrafanaLokiV3{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (p projectGrafanaLokiV3) Install(ctx context.Context, env *environment.Env) error {
	err := p.install(ctx, env, p.appPathCurrentVersion)
	return err
}

func (p projectGrafanaLokiV3) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	err := p.install(ctx, env, p.appPathPreviousVersion)
	return err
}

func (p projectGrafanaLokiV3) install(ctx context.Context, env *environment.Env, appPath string) error {
	// Create the project-level OBC secret for S3 credentials.
	if err := p.createOBCSecret(ctx, env); err != nil {
		return fmt.Errorf("creating OBC secret: %w", err)
	}

	// Deploy MinIO as a test S3 backend (required for distributed mode)
	if err := p.deployMinIO(ctx, env); err != nil {
		return fmt.Errorf("deploying MinIO: %w", err)
	}

	// Wait for MinIO to be ready and buckets to be created
	if err := p.waitForMinIOReady(ctx, env); err != nil {
		return fmt.Errorf("waiting for MinIO readiness: %w", err)
	}

	// Apply test overrides ConfigMap (MinIO S3, single ingester, etc.)
	if err := p.applyTestOverrides(ctx, env); err != nil {
		return fmt.Errorf("applying test overrides: %w", err)
	}

	// Apply only the helmrelease subdirectory which contains the raw
	// OCIRepository, HelmRelease, and ConfigMap (not the Flux Kustomization
	// wrappers which require a GitRepository that doesn't exist in test clusters).
	helmreleasePath := filepath.Join(appPath, "helmrelease")
	err := env.ApplyKustomizations(ctx, helmreleasePath, map[string]string{
		"appVersion":         "app-version-project-grafana-loki-v3",
		"releaseName":        "project-grafana-loki-v3",
		"releaseNamespace":   kommanderNamespace,
		"workspaceNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	// Patch the HelmRelease to include the test overrides ConfigMap
	if err := p.patchHelmReleaseWithOverrides(ctx, env); err != nil {
		return fmt.Errorf("patching HelmRelease with overrides: %w", err)
	}

	return nil
}

func (p projectGrafanaLokiV3) createOBCSecret(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	secretName := fmt.Sprintf("proj-loki-%s", kommanderNamespace)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: kommanderNamespace,
		},
		StringData: map[string]string{
			"BUCKET_HOST":           "localhost",
			"BUCKET_PORT":           "80",
			"BUCKET_NAME":           secretName,
			"AWS_ACCESS_KEY_ID":     "test-access-key",
			"AWS_SECRET_ACCESS_KEY": "test-secret-key",
		},
	}

	return ctrlClient.IgnoreAlreadyExists(client.Create(ctx, secret))
}

func (p projectGrafanaLokiV3) deployMinIO(ctx context.Context, env *environment.Env) error {
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

func (p projectGrafanaLokiV3) applyTestOverrides(ctx context.Context, env *environment.Env) error {
	testDataPath, err := getTestDataDir()
	if err != nil {
		return err
	}

	overridePath := filepath.Join(testDataPath, "grafana-loki-v3", "project-overrides.yaml")
	content, err := os.ReadFile(overridePath)
	if err != nil {
		return fmt.Errorf("failed to read test overrides: %w", err)
	}

	return env.ApplyYAMLFileRaw(ctx, content, map[string]string{
		"namespace": kommanderNamespace,
	})
}

func (p projectGrafanaLokiV3) patchHelmReleaseWithOverrides(ctx context.Context, env *environment.Env) error {
	hr := &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name(),
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
		Name: "project-grafana-loki-v3-test-overrides",
	})
	err = genericClient.Update(ctx, hr)
	if err != nil {
		return fmt.Errorf("could not update the HelmRelease: %w", err)
	}

	return nil
}

func (p projectGrafanaLokiV3) waitForMinIOReady(ctx context.Context, env *environment.Env) error {
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

func (p projectGrafanaLokiV3) Upgrade(ctx context.Context, env *environment.Env) error {
	return fmt.Errorf("upgrade is not yet implemented")
}
