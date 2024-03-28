package appscenarios

import (
	"fmt"
	"time"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Reloader Install Test", Ordered, Label("reloader", "install"), func() {

	It("should install successfully with default config", func() {
		r := reloader{}
		err := r.Install(ctx, env)
		Expect(err).To(BeNil())

		hr := &fluxhelmv2beta1.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       fluxhelmv2beta1.HelmReleaseKind,
				APIVersion: fluxhelmv2beta1.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.Name(),
				Namespace: kommanderNamespace,
			},
		}

		Eventually(func() error {
			err = k8sClient.Get(ctx, genericCLient.ObjectKeyFromObject(hr), hr)
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

	// Test reloads a simple test application appropriately

})

var _ = Describe("Reloader Upgrade Test", Ordered, Label("reloader", "upgrade"), func() {
	It("should return the name of the scenario", func() {
		r := reloader{}
		Expect(r.Name()).To(Equal("reloader1"))
	})
})
