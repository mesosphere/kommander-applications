// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/mesosphere/kommander-applications/client"
	"github.com/mesosphere/kommander-applications/docker"
	"github.com/mesosphere/kommander-applications/environment"
	"github.com/mesosphere/kommander-applications/flux"
	"github.com/mesosphere/kommander-applications/kind"
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

// InitSuite registers the -app-version flag and a Ginkgo BeforeSuite that
// initialises the cluster connection (E2E_KUBECONFIG) or Docker network
// (Kind). Call this from your suite's init() function.
func InitSuite() {
	AppVersion = flag.String("app-version", "", "The version of the application (required)")

	var _ = ginkgo.BeforeSuite(func() {
		gomega.Expect(*AppVersion).ToNot(gomega.BeEmpty(), "-app-version flag is required")

		log.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))
		Ctx = context.Background()

		if kubeconfig := os.Getenv("E2E_KUBECONFIG"); kubeconfig != "" {
			UseExistingCluster = true
			Env = &environment.Env{}

			typedClient, err := client.NewClient(kubeconfig)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			Env.K8sClient = typedClient

			scheme := flux.NewScheme()
			_ = fluxhelmv2.AddToScheme(scheme)

			K8sClient, err = genericClient.New(typedClient.Config(), genericClient.Options{Scheme: scheme})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		} else {
			var err error
			network, err = kind.EnsureDockerNetworkExist(Ctx, "", false)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			Env = &environment.Env{
				Network: network,
			}
		}
	})
}

// RunSuite is the standard Ginkgo test entry point. Call this from your
// suite's TestApplications function.
func RunSuite(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	ginkgo.RunSpecs(t, "Application Test Suite", suiteConfig, reporterConfig)
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
				ginkgo.GinkgoWriter.Printf("Waiting for API %s/%s: %v\n", r.group, r.version, err)
				allFound = false
				break
			}
		}
		if allFound {
			ginkgo.GinkgoWriter.Printf("All Flux CRDs are discoverable, refreshing clients\n")
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
