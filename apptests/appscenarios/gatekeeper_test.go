package appscenarios

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
)

var _ = Describe("GateKeeper Tests", Label("gatekeeper"), func() {
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
				Expect(gateKeeperContainer.Resources.Limits.Cpu().String()).To(Equal("0"))
				Expect(gateKeeperContainer.Resources.Limits.Memory().String()).To(Equal("512Mi"))
			}
		})

		It("should enforce the default constraints", func() {
			By("creating Project NS")
			projectNS := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-ns",
					Labels: map[string]string{
						"kommander.d2iq.io/managed-by-kind": "Project",
					},
				},
			}
			err := k8sClient.Create(ctx, projectNS)
			Expect(err).ToNot(HaveOccurred())
			ensureConstraintEnforced(projectNS.Name)
		})

	})

	Describe("GateKeeper Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			gateKeeperHr *fluxhelmv2beta2.HelmRelease
			projectNS    *corev1.Namespace
		)

		It("should install previous version successfully with default config", func() {
			err := gk.InstallPreviousVersion(ctx, env)
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

		It("should enforce the default constraints", func() {
			By("creating Project NS")
			projectNS = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-ns",
					Labels: map[string]string{
						"kommander.d2iq.io/managed-by-kind": "Project",
					},
				},
			}
			err := k8sClient.Create(ctx, projectNS)
			Expect(err).ToNot(HaveOccurred())
			ensureConstraintEnforced(projectNS.Name)
		})

		It("should upgrade gatekeeper successfully", func() {
			err := gk.Install(ctx, env)
			Expect(err).ToNot(HaveOccurred())

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

		It("should enforce the default constraints after upgrade", func() {
			ensureConstraintEnforced(projectNS.Name)
		})
	})
})

func ensureConstraintEnforced(projectNS string) {
	By("should require service account name defined in HelmRelease in Project")
	hr1 := &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hr-to-be-rejected",
			Namespace: projectNS, // we are treating this as a project NS
		},
		Spec: fluxhelmv2beta2.HelmReleaseSpec{
			Chart: fluxhelmv2beta2.HelmChartTemplate{
				Spec: fluxhelmv2beta2.HelmChartTemplateSpec{
					Chart:   "external-dns",
					Version: "7.2.0",
					SourceRef: fluxhelmv2beta2.CrossNamespaceObjectReference{
						Kind:      "HelmRepository",
						Name:      "charts.github.io-bitnami",
						Namespace: "kommander-flux",
					},
				},
			},
			Interval: metav1.Duration{Duration: 3 * time.Second},
		},
	}
	err := k8sClient.Create(ctx, hr1)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("admission webhook \"validation.gatekeeper.sh\" denied the request: [helmrelease-must-have-sa] must have a serviceAccountName set"))
	// not asserting kustomization enforcement since that needs a GitRepository
}
