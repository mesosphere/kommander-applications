package appscenarios

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/mesosphere/kommander-applications/apptests/docker"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"
)

var (
	env              *environment.Env
	ctx              context.Context
	network          *docker.NetworkResource
	k8sClient        genericClient.Client
	restClientV1Pods rest.Interface
	// Multi-cluster test variables (uses the same Env struct with workload fields populated)
	multiEnv                 *environment.Env
	workloadK8sClient        genericClient.Client
	workloadRestClientV1Pods rest.Interface
	workspaceNS              *corev1.Namespace
)

var _ = BeforeSuite(func() {
	ctx = context.Background()
	var err error
	network, err = kind.EnsureDockerNetworkExist(ctx, "", false)
	Expect(err).ShouldNot(HaveOccurred())

	Expect(err).ShouldNot(HaveOccurred())

	env = &environment.Env{
		Network: network,
	}

	// Get the path to upgrade k-apps repository from the environment variable
	upgradeKAppsRepoPath = os.Getenv(upgradeKappsRepoPathEnv)
	if upgradeKAppsRepoPath == "" {
		upgradeKAppsRepoPath = defaultUpgradeKAppsRepoPath
	}
})

func TestApplications(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Application Test Suite", suiteConfig, reporterConfig)
}

func SetupKindCluster() error {
	if ctx == nil {
		ctx = context.Background()
	}

	err := env.Provision(ctx)
	if err != nil {
		return err
	}

	k8sClient, err = genericClient.New(env.K8sClient.Config(), genericClient.Options{Scheme: flux.NewScheme()})
	if err != nil {
		return err
	}

	// Get a REST client for making http requests to pods
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	httpClient, err := rest.HTTPClientFor(env.K8sClient.Config())
	if err != nil {
		return err
	}

	restClientV1Pods, err = apiutil.RESTClientForGVK(
		gvk,
		false,
		false,
		env.K8sClient.Config(),
		serializer.NewCodecFactory(flux.NewScheme()),
		httpClient,
	)
	if err != nil {
		return err
	}

	return nil
}

// SetupMultiCluster provisions a multi-cluster environment with management and workload clusters.
// This function should be called in BeforeEach(OncePerOrdered, ...) for tests labeled with "multicluster".
func SetupMultiCluster() error {
	if ctx == nil {
		ctx = context.Background()
	}
	multiEnv = &environment.Env{}
	err := multiEnv.ProvisionMultiCluster(ctx)
	if err != nil {
		return err
	}

	k8sClient, err = genericClient.New(
		multiEnv.K8sClient.Config(),
		genericClient.Options{Scheme: flux.NewScheme()},
	)
	if err != nil {
		return err
	}

	workloadK8sClient, err = genericClient.New(
		multiEnv.WorkloadK8sClient.Config(),
		genericClient.Options{Scheme: flux.NewScheme()},
	)
	if err != nil {
		return err
	}

	// Get a REST client for making http requests to pods
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	mgmtHttpClient, err := rest.HTTPClientFor(multiEnv.K8sClient.Config())
	if err != nil {
		return err
	}

	restClientV1Pods, err = apiutil.RESTClientForGVK(
		gvk,
		false,
		false,
		multiEnv.K8sClient.Config(),
		serializer.NewCodecFactory(flux.NewScheme()),
		mgmtHttpClient,
	)
	if err != nil {
		return err
	}

	workloadHttpClient, err := rest.HTTPClientFor(multiEnv.WorkloadK8sClient.Config())
	if err != nil {
		return err
	}
	workloadRestClientV1Pods, err = apiutil.RESTClientForGVK(gvk,
		false,
		false,
		multiEnv.WorkloadK8sClient.Config(),
		serializer.NewCodecFactory(flux.NewScheme()),
		workloadHttpClient,
	)

	return nil
}

// TeardownMultiCluster destroys the multi-cluster environment.
// This function should be called in AfterEach(OncePerOrdered, ...) for tests labeled with "multicluster".
func TeardownMultiCluster() error {
	if multiEnv != nil {
		return multiEnv.DestroyMultiCluster(ctx)
	}
	return nil
}
