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

var _ = Describe("External DNS Tests", Label("external-dns"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())
	})

	AfterEach(OncePerOrdered, func() {
		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Installing external-dns", Ordered, Label("install"), func() {

		var (
			ed             externalDns
			hr             *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install successfully with default config", func() {
			ed = externalDns{}
			err := ed.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ed.Name(),
					Namespace: kommanderNamespace,
				},
			}

			// Check the status of the HelmReleases
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

		It("should have a PriorityClass configured", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": ed.Name(),
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

			for _, deployment := range deploymentList.Items {
				Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal(dkpHighPriority))
			}
		})

		// Check that metrics server on port 7979 on the external DNS pod is responding
		It("should emit metrics", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "external-dns",
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			podList := &corev1.PodList{}
			err = k8sClient.List(ctx, podList, listOptions)
			Expect(err).To(BeNil())
			Expect(podList.Items).To(HaveLen(1))

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":7979").SubResource("proxy").Suffix("metrics").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("external_dns"))
		})
	})

	Describe("Upgrading external-dns", Ordered, Label("upgrade"), func() {
		var (
			ed externalDns
			hr *fluxhelmv2beta2.HelmRelease
		)

		It("should install the previous version successfully", func() {
			ed = externalDns{}
			err := ed.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ed.Name(),
					Namespace: kommanderNamespace,
				},
			}

			// Check the status of the HelmReleases
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

		It("should emit metrics", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "external-dns",
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			podList := &corev1.PodList{}
			err = k8sClient.List(ctx, podList, listOptions)
			Expect(err).To(BeNil())
			Expect(podList.Items).To(HaveLen(1))

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":7979").SubResource("proxy").Suffix("metrics").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("external_dns"))
		})

		It("should upgrade external-dns successfully", func() {
			err := ed.Upgrade(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ed.Name(),
					Namespace: kommanderNamespace,
				},
			}

			// Check the status of the HelmReleases
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

		It("should emit metrics", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "external-dns",
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			podList := &corev1.PodList{}
			err = k8sClient.List(ctx, podList, listOptions)
			Expect(err).To(BeNil())
			Expect(podList.Items).To(HaveLen(1))

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":7979").SubResource("proxy").Suffix("metrics").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("external_dns"))
		})

	})
})
