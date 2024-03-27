package apptests_test

import (
	"context"
	"testing"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {
	env := &environment.Env{}
	ctx := context.Background()

	Describe("Creating kind cluster", func() {
		It("should create a kind cluster", func() {
			err := env.Provision(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})

})

func TestApptests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Application Test Suite")
}

var _ = AfterSuite(func() {
	env := &environment.Env{}
	ctx := context.Background()

	Describe("Destroying kind cluster", func() {
		It("should destroy the kind cluster", func() {
			err := env.Destroy(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
