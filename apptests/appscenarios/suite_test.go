package appscenarios

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mesosphere/kommander-applications/apptests/docker"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/kind"
	"github.com/mesosphere/kommander-applications/apptests/net"
	"k8s.io/client-go/rest"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	env                  *environment.Env
	ctx                  context.Context
	network              *docker.NetworkResource
	subnet               *net.Subnet
	k8sClient            genericClient.Client
	restClientV1Pods     rest.Interface
	upgradeKAppsRepoPath string
)

var _ = BeforeSuite(func() {
	ctx = context.Background()
	var err error
	network, err = kind.EnsureDockerNetworkExist(ctx, "", false)
	Expect(err).ShouldNot(HaveOccurred())

	subnet, err = network.Subnet()
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
