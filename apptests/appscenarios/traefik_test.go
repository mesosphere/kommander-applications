package appscenarios

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	traefikv1a1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
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
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Traefik Install Test", Ordered, Label("install"), func() {
		var (
			hr             *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
			podList        *corev1.PodList
		)

		It("should install successfully with default config", func() {
			err := t.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
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
			deploymentList = &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, deploymentList, listOptions)
			Expect(err).To(BeNil())
			Expect(deploymentList.Items).To(HaveLen(1))
		})

		It("should create middlewares", func() {
			middlewareList := &traefikv1a1.MiddlewareList{}
			err := k8sClient.List(ctx, middlewareList, &ctrlClient.ListOptions{
				Namespace: kommanderNamespace,
			})
			Expect(err).To(BeNil())
			Expect(middlewareList.Items).To(HaveLen(5))
			Expect(middlewareList.Items).To(WithTransform(func(mwList []traefikv1a1.Middleware) []string {
				var names []string
				for _, mw := range mwList {
					names = append(names, mw.Name)
				}
				return names
			}, ContainElements("stripprefixes", "stripprefixes-kubetunnel", "forwardauth", "forwardauth-full", "rewrite-api")))

		})

		It("should create dashboard ingress route", func() {
			ingressRouteList := &traefikv1a1.IngressRouteList{}
			err := k8sClient.List(ctx, ingressRouteList, &ctrlClient.ListOptions{
				Namespace: kommanderNamespace,
			})
			Expect(err).To(BeNil())
			Expect(ingressRouteList.Items).To(HaveLen(1))
			Expect(ingressRouteList.Items[0].Name).To(Equal(fmt.Sprintf("%s-dashboard", hr.GetReleaseName())))
			Expect(ingressRouteList.Items[0].Annotations).To(HaveKeyWithValue("kubernetes.io/ingress.class", hr.GetReleaseName()))
		})

		It("should have access to multiple traefik endpoints", func() {
			podList = &corev1.PodList{}
			assertTraefikEndpoints(t, podList)
		})

	})

	Describe("Traefik Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			hr      *fluxhelmv2beta2.HelmRelease
			podList *corev1.PodList
		)

		It("should install the previous version successfully", func() {
			err := t.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      t.Name(),
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

		It("should have access to multiple traefik endpoints", func() {
			podList = &corev1.PodList{}
			assertTraefikEndpoints(t, podList)
		})

		It("should upgrade traefik successfully", func() {
			err := t.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      t.Name(),
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

		It("should have access to multiple traefik endpoints after upgrade", func() {
			assertTraefikEndpoints(t, podList)
		})
	})
})

func assertTraefikEndpoints(t *traefik, podList *corev1.PodList) {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": t.Name(),
		},
	})
	Expect(err).To(BeNil())
	listOptions := &ctrlClient.ListOptions{
		LabelSelector: selector,
	}

	Eventually(func() ([]corev1.Pod, error) {
		err := k8sClient.List(ctx, podList, listOptions)
		return podList.Items, err
	}).WithPolling(5 * time.Second).WithTimeout(time.Minute).Should(HaveLen(1))

	By("triggering metrics generation on port 8443")
	res := restClientV1Pods.Get().Resource("pods").
		Namespace(podList.Items[0].Namespace).
		Name(podList.Items[0].Name + ":8443").
		SubResource("proxy").
		Do(ctx)
	Expect(res.Error()).To(BeNil())

	var statusCode int
	res.StatusCode(&statusCode)
	Expect(statusCode).To(Equal(200))

	// Check if the body contains expected content (if any)
	body, err := res.Raw()
	Expect(err).To(BeNil())
	Expect(string(body)).To(ContainSubstring("metrics generation triggered"))

	By("checking traefik prometheus metrics endpoint")
	res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":9100").SubResource("proxy").Suffix("/metrics").Do(ctx)
	Expect(res.Error()).To(BeNil())

	var statusCode int
	res.StatusCode(&statusCode)
	Expect(statusCode).To(Equal(200))

	body, err := res.Raw()
	Expect(err).To(BeNil())
	Expect(string(body)).To(ContainSubstring("traefik_entrypoint_requests_total"))

	By("checking traefik dashboard endpoint")
	res = restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":9000").SubResource("proxy").Suffix("/dashboard/").Do(ctx)
	Expect(res.Error()).To(BeNil())

	res.StatusCode(&statusCode)
	Expect(statusCode).To(Equal(200))

	body, err = res.Raw()
	Expect(err).To(BeNil())
	Expect(string(body)).To(ContainSubstring("Traefik UI"))
}
