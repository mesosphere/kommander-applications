package appscenarios

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
)

const (
	gatekeeperProxyMutationsHRName = "gatekeeper-proxy-mutations"
	mutatingWebhookConfigName      = "gatekeeper-mutating-webhook-configuration"
	defaultSeccompAssignName       = "pod-seccomp-runtime-default"
	defaultSeccompOptOutLabel      = "kommander.d2iq.io/disable-default-seccomp"
)

var _ = Describe("GateKeeper Tests", Label("gatekeeper"), func() {
	var gk *gatekeeper

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
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("GateKeeper Install Test", Ordered, Label("install"), func() {
		var (
			gateKeeperHr             *fluxhelmv2.HelmRelease
			gateKeeperDeploymentList *appsv1.DeploymentList
			gateKeeperContainer      corev1.Container
		)

		It("should install successfully with default config", func() {
			err := gk.Install(ctx, env)
			Expect(err).ToNot(HaveOccurred())

			gateKeeperHr = &fluxhelmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2.HelmReleaseKind,
					APIVersion: fluxhelmv2.GroupVersion.Version,
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
			for i := range gateKeeperDeploymentList.Items {
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

		It("should install the mutating webhook configuration", func() {
			ensureMutatingWebhookConfiguration()
		})

		It("should mutate pods without a seccomp profile to RuntimeDefault", func() {
			By("waiting for the gatekeeper-proxy-mutations HelmRelease to be ready")
			waitForHelmReleaseReady(gatekeeperProxyMutationsHRName)

			By("waiting for the default seccomp Assign to be created by the chart")
			waitForDefaultSeccompAssign()

			ns := createNamespace("seccomp-default-test", nil)
			DeferCleanup(deleteNamespace, ns)

			pod := newPauseSeccompTestPod(ns.Name, "seccomp-default-pod")
			admittedPod := createPodAndRefetch(pod)

			Expect(admittedPod.Spec.SecurityContext).ToNot(BeNil(),
				"expected pod-level securityContext to be added by the Assign mutation")
			Expect(admittedPod.Spec.SecurityContext.SeccompProfile).ToNot(BeNil(),
				"expected seccompProfile to be set by the Assign mutation")
			Expect(admittedPod.Spec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
		})

		It("should not mutate pods in namespaces labeled with the opt-out label", func() {
			By("waiting for the default seccomp Assign to be created by the chart")
			waitForDefaultSeccompAssign()

			ns := createNamespace("seccomp-default-test-optout", map[string]string{
				defaultSeccompOptOutLabel: "true",
			})
			DeferCleanup(deleteNamespace, ns)

			pod := newPauseSeccompTestPod(ns.Name, "seccomp-default-pod-optout")
			admittedPod := createPodAndRefetch(pod)

			if admittedPod.Spec.SecurityContext != nil {
				Expect(admittedPod.Spec.SecurityContext.SeccompProfile).To(BeNil(),
					"expected Assign mutation to be skipped in opt-out namespace")
			}
		})
	})

	Describe("GateKeeper Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			gateKeeperHr *fluxhelmv2.HelmRelease
			projectNS    *corev1.Namespace
		)

		It("should install previous version successfully with default config", func() {
			err := gk.InstallPreviousVersion(ctx, env)
			Expect(err).ToNot(HaveOccurred())

			gateKeeperHr = &fluxhelmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2.HelmReleaseKind,
					APIVersion: fluxhelmv2.GroupVersion.Version,
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

		It("should have the mutating webhook configuration after upgrade", func() {
			ensureMutatingWebhookConfiguration()
		})
	})
})

func ensureMutatingWebhookConfiguration() {
	mwc := &unstructured.Unstructured{}
	mwc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "MutatingWebhookConfiguration",
	})

	Eventually(func() error {
		return k8sClient.Get(ctx, ctrlClient.ObjectKey{Name: mutatingWebhookConfigName}, mwc)
	}).WithPolling(pollInterval).WithTimeout(2 * time.Minute).Should(Succeed(),
		"expected MutatingWebhookConfiguration %q to exist after install", mutatingWebhookConfigName)

	webhooks, found, err := unstructured.NestedSlice(mwc.Object, "webhooks")
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "expected MutatingWebhookConfiguration to expose webhooks")
	Expect(webhooks).ToNot(BeEmpty(), "expected at least one webhook entry")
}

func waitForHelmReleaseReady(name string) {
	hr := &fluxhelmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2.HelmReleaseKind,
			APIVersion: fluxhelmv2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kommanderNamespace,
		},
	}
	Eventually(func() error {
		if err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr); err != nil {
			return err
		}
		for _, cond := range hr.Status.Conditions {
			if cond.Status == metav1.ConditionTrue && cond.Type == apimeta.ReadyCondition {
				return nil
			}
		}
		return fmt.Errorf("HelmRelease %q not ready yet", name)
	}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
}

func waitForDefaultSeccompAssign() {
	assign := &unstructured.Unstructured{}
	assign.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "mutations.gatekeeper.sh",
		Version: "v1",
		Kind:    "Assign",
	})
	Eventually(func() error {
		return k8sClient.Get(ctx, ctrlClient.ObjectKey{Name: defaultSeccompAssignName}, assign)
	}).WithPolling(pollInterval).WithTimeout(2 * time.Minute).Should(Succeed(),
		"expected default seccomp Assign %q to be created by the gatekeeper-proxy-mutations chart", defaultSeccompAssignName)
}

func createNamespace(name string, labels map[string]string) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	return ns
}

func deleteNamespace(ns *corev1.Namespace) {
	if err := k8sClient.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}
}

func newPauseSeccompTestPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "pause",
				Image: "registry.k8s.io/pause:3.9",
			}},
			TerminationGracePeriodSeconds: ptrInt64(0),
		},
	}
}

func createPodAndRefetch(pod *corev1.Pod) *corev1.Pod {
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	out := &corev1.Pod{}
	Expect(k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(pod), out)).To(Succeed())
	return out
}

func ptrInt64(v int64) *int64 { return &v }

func ensureConstraintEnforced(projectNS string) {
	By("should require service account name defined in HelmRelease in Project")
	hr1 := &fluxhelmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2.HelmReleaseKind,
			APIVersion: fluxhelmv2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hr-to-be-rejected",
			Namespace: projectNS, // we are treating this as a project NS
		},
		Spec: fluxhelmv2.HelmReleaseSpec{
			ChartRef: &fluxhelmv2.CrossNamespaceSourceReference{
				Kind:      "HelmChart",
				Name:      "external-dns",
				Namespace: projectNS,
			},
			Interval: metav1.Duration{Duration: 3 * time.Second},
		},
	}
	err := k8sClient.Create(ctx, hr1)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("admission webhook \"validation.gatekeeper.sh\" denied the request: [helmrelease-must-have-sa] must have a serviceAccountName set"))
	// not asserting kustomization enforcement since that needs a GitRepository
}
