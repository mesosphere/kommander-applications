package appscenarios

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	env                  *environment.Env
	ctx                  context.Context
	k8sClient            genericClient.Client
	upgradeKAppsRepoPath string
)

var _ = BeforeSuite(func() {
	env = &environment.Env{}
	ctx = context.Background()

	err := env.Provision(ctx)
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err = genericClient.New(env.K8sClient.Config(), genericClient.Options{Scheme: flux.NewScheme()})
	Expect(k8sClient).ToNot(BeNil())
	Expect(err).To(BeNil())

	// Get the path to upgrade k-apps repository from the environment variable
	upgradeKAppsRepoPath = os.Getenv("UPGRADE_KAPPS_REPO_PATH")
	if upgradeKAppsRepoPath == "" {
		upgradeKAppsRepoPath = defaultUpgradeKAppsRepoPath
	}
})

var _ = AfterSuite(func() {
	err := env.Destroy(ctx)
	Expect(err).ToNot(HaveOccurred())
})

func TestApplications(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Application Test Suite", suiteConfig, reporterConfig)
}
