package appscenarios

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
)

var _ = Describe("Traefik Tests", Label("traefik"), func() {
	var (
		t *traefik
	)

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		t = NewTraefik()
	})

	AfterEach(OncePerOrdered, func() {
		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Traefik Install Test", Ordered, Label("install"), func() {
		var (
			traefikHr             *fluxhelmv2beta2.HelmRelease
			traefikDeploymentList *appsv1.DeploymentList
			//traefikContainer      corev1.Container
		)

		It("should install successfully with default config", func() {
			err := t.Install(ctx, env)
			Expect(err).To(BeNil())

			traefikHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      t.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(traefikHr), traefikHr)
				if err != nil {
					return err
				}

				for _, cond := range traefikHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should have resource limits and priority class", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": t.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			traefikDeploymentList = &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, traefikDeploymentList, listOptions)
			Expect(err).To(BeNil())
			Expect(traefikDeploymentList.Items).To(HaveLen(1))
		})
	})
})
