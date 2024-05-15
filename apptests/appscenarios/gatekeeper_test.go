package appscenarios

import (
	"fmt"
	"time"

	apimeta "github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
)

var _ = Describe("GateKeeper Tests", func() {
	var (
		gk *gatekeeper
	)

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).ToNot(HaveOccurred())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		gk = NewGatekeeper()
	})

	AfterEach(OncePerOrdered, func() {
		err := env.Destroy(ctx)
		Expect(err).To(BeNil())
	})

	Describe("GateKeeper Install Test", Ordered, Label("install"), func() {
		var (
			gateKeeperHr             *fluxhelmv2beta2.HelmRelease
			gateKeeperDeploymentList *appsv1.DeploymentList
			gateKeeperContainer      corev1.Container
		)

		It("should install successfully with default config", func() {
			err := gk.Install(ctx, env)
			Expect(err).ToNot(HaveOccurred())

			gateKeeperHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      gk.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(gateKeeperHr), gateKeeperHr)
				if err != nil {
					return err
				}

				for _, cond := range gateKeeperHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		Context("GateKeeper Deployments", func() {
			It("should have resource limits and priority class set", func() {
				selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
					MatchLabels: map[string]string{
						"helm.toolkit.fluxcd.io/name": gk.Name(),
					},
				})
				Expect(err).ToNot(HaveOccurred())
				listOptions := &ctrlClient.ListOptions{
					LabelSelector: selector,
				}
				gateKeeperDeploymentList = &appsv1.DeploymentList{}
				err = k8sClient.List(ctx, gateKeeperDeploymentList, listOptions)
				Expect(err).To(BeNil())
				Expect(len(gateKeeperDeploymentList.Items)).To(Equal(2))
				for i, _ := range gateKeeperDeploymentList.Items {
					Expect(gateKeeperDeploymentList.Items[i].Spec.Template.Spec.PriorityClassName).To(Equal(systemClusterCriticalPriority))
					gateKeeperContainer = gateKeeperDeploymentList.Items[i].Spec.Template.Spec.Containers[0]
					Expect(gateKeeperContainer.Resources.Requests.Cpu().String()).To(Equal("100m"))
					Expect(gateKeeperContainer.Resources.Requests.Memory().String()).To(Equal("512Mi"))
					Expect(gateKeeperContainer.Resources.Limits.Cpu()).To(BeEmpty())
					Expect(gateKeeperContainer.Resources.Limits.Memory().String()).To(Equal("512Mi"))
				}
			})

		})

	})

	Describe("GateKeeper Upgrade Test", Ordered, Label("upgrade"), func() {

	})

})
