package appscenarios

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Installing kommander-flux", Ordered, Label("kommander-flux", "install"), func() {

	var (
		kf             kommanderFlux
		deploymentList *appsv1.DeploymentList
	)

	It("should install successfully with default config", func() {
		kf = kommanderFlux{}
		err := kf.Install(ctx, env)
		Expect(err).To(BeNil())

		// Check the status of the flux deployments
		Eventually(func() error {
			deploymentList := &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, deploymentList, ctrlClient.MatchingLabels{"app.kubernetes.io/instance": kf.Name()})
			if err != nil {
				return err
			}

			Expect(deploymentList.Items).To(HaveLen(4))
			Expect(err).To(BeNil())

			for _, deployment := range deploymentList.Items {
				if deployment.Status.ReadyReplicas == 0 {
					return fmt.Errorf("deployment not ready yet")
				}
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should have a PriorityClass configured on all 4 deployments", func() {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance": kf.Name(),
			},
		})
		Expect(err).To(BeNil())
		listOptions := &ctrlClient.ListOptions{
			LabelSelector: selector,
		}
		deploymentList = &appsv1.DeploymentList{}
		err = k8sClient.List(ctx, deploymentList, listOptions)
		Expect(err).To(BeNil())
		Expect(deploymentList.Items).To(HaveLen(4))

		for _, deployment := range deploymentList.Items {
			Expect(deployment.Spec.Template.Spec.PriorityClassName).ToNot(BeNil())
		}
	})

})

var _ = Describe("Upgrading komander-flux", Ordered, Label("kommander-flux", "upgrade"), func() {
	var (
		kf kommanderFlux
	)

	It("should install the previous version successfully", func() {
		kf = kommanderFlux{}
		err := kf.InstallPreviousVersion(ctx, env)
		Expect(err).To(BeNil())

		// Check the status of the flux deployments
		Eventually(func() error {
			deploymentList := &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, deploymentList, ctrlClient.MatchingLabels{"app.kubernetes.io/instance": kf.Name()})
			if err != nil {
				return err
			}

			for _, deployment := range deploymentList.Items {
				if deployment.Status.ReadyReplicas == 0 {
					return fmt.Errorf("deployment not ready yet")
				}
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should upgrade flux successfully", func() {
		err := kf.Upgrade(ctx, env)
		Expect(err).To(BeNil())

		// Check the status of the flux deployments
		Eventually(func() error {
			deploymentList := &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, deploymentList, ctrlClient.MatchingLabels{"app.kubernetes.io/instance": kf.Name()})
			if err != nil {
				return err
			}

			for _, deployment := range deploymentList.Items {
				if deployment.Status.ReadyReplicas == 0 {
					return fmt.Errorf("deployment not ready yet")
				}
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})
})
