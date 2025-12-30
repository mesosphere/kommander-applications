package appscenarios

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

var _ = Describe("Multi-Cluster OpenCost Tests", Label("opencost"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupMultiCluster()
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.InstallLatestFlux(ctx)
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.InstallLatestFluxOnWorkload(ctx)
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.ApplyKommanderPriorityClasses(ctx, environment.ManagementClusterTarget)
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.ApplyKommanderPriorityClasses(ctx, environment.WorkloadClusterTarget)
		Expect(err).To(Not(HaveOccurred()))
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := TeardownMultiCluster()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("test", Ordered, Label("install"), func() {
		It("should setup multi-cluster environment", func() {
			Expect(multiEnv).ToNot(BeNil())
			Expect(managementK8sClient).ToNot(BeNil())
			Expect(workloadK8sClient).ToNot(BeNil())
		})
	})
})
