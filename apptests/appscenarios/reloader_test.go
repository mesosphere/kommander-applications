package appscenarios

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Reloader Install Test", Ordered, Label("reloader", "install"), func() {

	var (
		r                 reloader
		hr                *fluxhelmv2beta2.HelmRelease
		deploymentList    *appsv1.DeploymentList
		reloaderContainer corev1.Container
	)

	It("should install successfully with default config", func() {
		r = reloader{}
		err := r.Install(ctx, env)
		Expect(err).To(BeNil())

		hr = &fluxhelmv2beta2.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       fluxhelmv2beta2.HelmReleaseKind,
				APIVersion: fluxhelmv2beta2.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.Name(),
				Namespace: kommanderNamespace,
			},
		}

		Eventually(func() error {
			err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
			if err != nil {
				return err
			}

			for _, cond := range hr.Status.Conditions {
				if cond.Status == metav1.ConditionTrue &&
					cond.Type == apimeta.ReadyCondition {
					return nil
				}
			}
			return fmt.Errorf("helm release not ready yet")
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	// Assert the existence of resource limits and priority class
	It("should have resource limits and priority class", func() {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"helm.toolkit.fluxcd.io/name": r.Name(),
			},
		})
		Expect(err).To(BeNil())
		listOptions := &ctrlClient.ListOptions{
			LabelSelector: selector,
		}
		deploymentList = &appsv1.DeploymentList{}
		err = k8sClient.List(ctx, deploymentList, listOptions)
		Expect(err).To(BeNil())
		Expect(deploymentList.Items).To(HaveLen(1))
		Expect(err).To(BeNil())

		Expect(deploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal("dkp-high-priority"))

		reloaderContainer = deploymentList.Items[0].Spec.Template.Spec.Containers[0]
		Expect(reloaderContainer.Resources.Requests.Cpu().String()).To(Equal("100m"))
		Expect(reloaderContainer.Resources.Requests.Memory().String()).To(Equal("128Mi"))
		Expect(reloaderContainer.Resources.Limits.Cpu().String()).To(Equal("100m"))
		Expect(reloaderContainer.Resources.Limits.Memory().String()).To(Equal("512Mi"))
	})
	// Test reloads a simple test application appropriately
	It("should reload the application", func() {
		// deploy a CM
		// deploy a deployment that use CM as env var
		// update the secret
		// check if the deployment is updated
	})

})

var _ = Describe("Reloader Upgrade Test", Ordered, Label("reloader", "upgrade"), func() {
	It("should return the name of the scenario", func() {
		r := reloader{}
		Expect(r.Name()).To(Equal("reloader"))
	})
})
