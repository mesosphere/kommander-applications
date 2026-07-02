// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/docker"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Exported suite state shared between the harness and per-app test files.
var (
	Env       *environment.Env
	Ctx       context.Context
	K8sClient genericClient.Client

	AppVersion         *string
	UseExistingCluster bool

	network *docker.NetworkResource
)

const fluxHelmControllerFeatureGatesEnv = "APPTESTS_HELM_CONTROLLER_FEATURE_GATES"

// InitSuite registers the -app-version flag and a Ginkgo BeforeSuite that
// initialises the cluster connection (E2E_KUBECONFIG) or Docker network
// (Kind). Call this from your suite's init() function.
func InitSuite() {
	AppVersion = flag.String("app-version", "", "The version of the application (required)")

	var _ = BeforeSuite(func() {
		Expect(*AppVersion).ToNot(BeEmpty(), "-app-version flag is required")

		log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		Ctx = context.Background()

		if kubeconfig := os.Getenv("E2E_KUBECONFIG"); kubeconfig != "" {
			UseExistingCluster = true
			Env = &environment.Env{}

			typedClient, err := client.NewClient(kubeconfig)
			Expect(err).ShouldNot(HaveOccurred())
			Env.K8sClient = typedClient

			scheme := flux.NewScheme()
			_ = fluxhelmv2.AddToScheme(scheme)

			K8sClient, err = genericClient.New(typedClient.Config(), genericClient.Options{Scheme: scheme})
			Expect(err).ShouldNot(HaveOccurred())
		} else {
			var err error
			network, err = kind.EnsureDockerNetworkExist(Ctx, "", false)
			Expect(err).ShouldNot(HaveOccurred())

			Env = &environment.Env{
				Network: network,
			}
		}
	})
}

// RunSuite is the standard Ginkgo test entry point. Call this from your
// suite's TestApplications function.
func RunSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Application Test Suite", suiteConfig, reporterConfig)
}

// SetupKindCluster provisions a Kind cluster with Flux, or is a no-op when
// an existing cluster is being used via E2E_KUBECONFIG. Safe to call from
// multiple Ordered containers -- only the first call provisions.
func SetupKindCluster() error {
	if UseExistingCluster {
		return nil
	}

	if Ctx == nil {
		Ctx = context.Background()
	}

	err := Env.Provision(Ctx)
	if err != nil {
		return err
	}

	scheme := flux.NewScheme()
	_ = fluxhelmv2.AddToScheme(scheme)

	K8sClient, err = genericClient.New(Env.K8sClient.Config(), genericClient.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	return nil
}

// ApplyFluxHelmControllerFeatureGates patches helm-controller args when
// APPTESTS_HELM_CONTROLLER_FEATURE_GATES is set (example:
// DisableChartDigestTracking=true). This allows shared test-harness control
// without per-suite custom patching.
func ApplyFluxHelmControllerFeatureGates() error {
	value := strings.TrimSpace(os.Getenv(fluxHelmControllerFeatureGatesEnv))
	if value == "" {
		value = "DisableChartDigestTracking=true"
	}
	if K8sClient == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	featureGateArg := value
	if !strings.HasPrefix(featureGateArg, "--feature-gates=") {
		featureGateArg = "--feature-gates=" + value
	}
	GinkgoWriter.Printf("Applying helm-controller feature gates: %s\n", featureGateArg)

	namespaces := []string{"flux-system", "kommander-flux"}
	var lastErr error
	for _, ns := range namespaces {
		key := types.NamespacedName{Name: "helm-controller", Namespace: ns}
		deploy := &appsv1.Deployment{}
		if err := K8sClient.Get(Ctx, key, deploy); err != nil {
			lastErr = err
			continue
		}
		if len(deploy.Spec.Template.Spec.Containers) == 0 {
			lastErr = fmt.Errorf("deployment %s/%s has no containers", ns, "helm-controller")
			continue
		}

		args := deploy.Spec.Template.Spec.Containers[0].Args
		for _, arg := range args {
			if arg == featureGateArg {
				GinkgoWriter.Printf("helm-controller in %s already has %s\n", ns, featureGateArg)
				return nil
			}
		}
		deploy.Spec.Template.Spec.Containers[0].Args = append(args, featureGateArg)
		if err := K8sClient.Update(Ctx, deploy); err != nil {
			lastErr = err
			continue
		}
		GinkgoWriter.Printf("Patched helm-controller in %s with %s\n", ns, featureGateArg)
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("failed to patch helm-controller feature gates: %w", lastErr)
	}
	return fmt.Errorf("helm-controller deployment not found in namespaces: %v", namespaces)
}

// WaitForFluxCRDs polls the API server until the Flux CRDs (HelmRelease,
// OCIRepository, Kustomization) are discoverable. Call after InstallLatestFlux
// to avoid racing the API server's discovery cache refresh.
func WaitForFluxCRDs() error {
	type gvr struct{ group, version, resource string }
	required := []gvr{
		{"helm.toolkit.fluxcd.io", "v2", "helmreleases"},
		{"source.toolkit.fluxcd.io", "v1", "ocirepositories"},
		{"kustomize.toolkit.fluxcd.io", "v1", "kustomizations"},
	}

	ctx, cancel := context.WithTimeout(Ctx, 2*time.Minute)
	defer cancel()

	for {
		allFound := true
		for _, r := range required {
			_, err := Env.K8sClient.Clientset().Discovery().
				ServerResourcesForGroupVersion(r.group + "/" + r.version)
			if err != nil {
				GinkgoWriter.Printf("Waiting for API %s/%s: %v\n", r.group, r.version, err)
				allFound = false
				break
			}
		}
		if allFound {
			GinkgoWriter.Printf("All Flux CRDs are discoverable, refreshing clients\n")
			scheme := flux.NewScheme()
			_ = fluxhelmv2.AddToScheme(scheme)
			c, err := genericClient.New(Env.K8sClient.Config(), genericClient.Options{Scheme: scheme})
			if err != nil {
				return fmt.Errorf("recreating client after CRD discovery: %w", err)
			}
			Env.SetClient(c)
			K8sClient = c
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for Flux CRDs to become available")
		case <-time.After(2 * time.Second):
		}
	}
}

// TeardownCluster destroys the Kind cluster unless UseExistingCluster
// or SKIP_CLUSTER_TEARDOWN is set.
func TeardownCluster() error {
	if UseExistingCluster || os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
		return nil
	}
	return Env.Destroy(Ctx)
}
