package appscenarios

import (
	"context"
	"testing"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	env *environment.Env
	ctx context.Context
)

var _ = BeforeSuite(func() {
	env = &environment.Env{}
	ctx = context.Background()

	err := env.Provision(ctx)
	Expect(err).ToNot(HaveOccurred())
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
