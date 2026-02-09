package appscenarios

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Grafana Loki v3 Tests", Label("grafana-loki-v3"), func() {
	var g *grafanaLokiV3

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		g = NewGrafanaLokiV3()
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Grafana Loki v3 Install Test", Ordered, Label("install"), func() {
		var (
			hr              *fluxhelmv2beta2.HelmRelease
			deploymentList  *appsv1.DeploymentList
			statefulSetList *appsv1.StatefulSetList
		)

		It("should install successfully with default config", func() {
			err := g.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      g.Name(),
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
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should have correct number of deployments with priority class", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			deploymentList = &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, deploymentList, listOptions)
			Expect(err).To(BeNil())

			// Expected deployments: distributor, querier, query-frontend, query-scheduler, gateway
			// May also include ruler if enabled
			Expect(len(deploymentList.Items)).To(BeNumerically(">=", 5))

			// Verify priority class for each deployment
			for _, deployment := range deploymentList.Items {
				Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Or(
					Equal(dkpCriticalPriority),
					Equal(dkpHighPriority),
				), fmt.Sprintf("deployment %s should have a priority class", deployment.Name))
			}
		})

		It("should have correct number of statefulsets with priority class", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			statefulSetList = &appsv1.StatefulSetList{}
			err = k8sClient.List(ctx, statefulSetList, listOptions)
			Expect(err).To(BeNil())

			// Expected statefulsets: ingester, index-gateway, compactor
			Expect(len(statefulSetList.Items)).To(BeNumerically(">=", 3))

			// Verify priority class for each statefulset
			for _, sts := range statefulSetList.Items {
				Expect(sts.Spec.Template.Spec.PriorityClassName).To(Or(
					Equal(dkpCriticalPriority),
					Equal(dkpHighPriority),
				), fmt.Sprintf("statefulset %s should have a priority class", sts.Name))
			}
		})

		It("should have all pods running and ready", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}

			Eventually(func() error {
				podList := &corev1.PodList{}
				err := k8sClient.List(ctx, podList, listOptions)
				if err != nil {
					return err
				}

				if len(podList.Items) == 0 {
					return fmt.Errorf("no loki pods found")
				}

				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("pod %s is not running, current phase: %s", pod.Name, pod.Status.Phase)
					}

					// Check if pod is ready
					ready := false
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							ready = true
							break
						}
					}
					if !ready {
						return fmt.Errorf("pod %s is not ready", pod.Name)
					}
				}

				return nil
			}).WithPolling(5 * time.Second).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should have gateway service accessible", func() {
			serviceList := &corev1.ServiceList{}
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
					"app.kubernetes.io/component": "gateway",
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}

			err = k8sClient.List(ctx, serviceList, listOptions)
			Expect(err).To(BeNil())
			Expect(len(serviceList.Items)).To(BeNumerically(">=", 1), "gateway service should exist")

			// Check if service is accessible via proxy
			podList := &corev1.PodList{}
			podSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
					"app.kubernetes.io/component": "gateway",
				},
			})
			Expect(err).To(BeNil())
			podListOptions := &ctrlClient.ListOptions{
				LabelSelector: podSelector,
			}

			Eventually(func() error {
				err := k8sClient.List(ctx, podList, podListOptions)
				if err != nil {
					return err
				}

				if len(podList.Items) == 0 {
					return fmt.Errorf("no gateway pods found")
				}

				// Try to access the gateway pod's ready endpoint
				ref := net.JoinSchemeNamePort("http", podList.Items[0].Name, "8080")
				res := restClientV1Pods.Get().
					Resource("pods").
					Namespace(podList.Items[0].Namespace).
					Name(ref).
					SubResource("proxy").
					Suffix("/ready").Do(ctx)

				return res.Error()
			}, "2m", "5s").Should(Succeed())
		})

		It("should verify resource limits are set for critical components", func() {
			// Check ingester statefulset
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
					"app.kubernetes.io/component": "ingester",
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}

			statefulSetList := &appsv1.StatefulSetList{}
			err = k8sClient.List(ctx, statefulSetList, listOptions)
			Expect(err).To(BeNil())
			Expect(len(statefulSetList.Items)).To(BeNumerically(">=", 1))

			ingesterContainer := statefulSetList.Items[0].Spec.Template.Spec.Containers[0]
			Expect(ingesterContainer.Resources.Requests).ToNot(BeNil(), "ingester should have resource requests")
			Expect(ingesterContainer.Resources.Limits).ToNot(BeNil(), "ingester should have resource limits")
		})
	})

	Describe("Grafana Loki v3 Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			hr *fluxhelmv2beta2.HelmRelease
		)

		It("should install the previous version successfully", func() {
			err := g.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      g.Name(),
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
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should upgrade grafana-loki-v3 successfully", func() {
			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      g.Name(),
					Namespace: kommanderNamespace,
				},
			}
			Expect(k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)).To(Succeed())
			existingGeneration := hr.Status.ObservedGeneration

			err := g.Install(ctx, env)
			Expect(err).To(BeNil())

			// Check the status of the HelmRelease
			Eventually(func() (*fluxhelmv2beta2.HelmRelease, error) {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				return hr, err
			}, "10m", pollInterval).Should(And(
				HaveField("Status.ObservedGeneration", BeNumerically(">=", existingGeneration)),
				HaveField("Status.Conditions", ContainElement(And(
					HaveField("Type", Equal(apimeta.ReadyCondition)),
					HaveField("Status", Equal(metav1.ConditionTrue)))),
				),
			))
		})

		It("should have all pods running and ready after upgrade", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": g.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}

			Eventually(func() error {
				podList := &corev1.PodList{}
				err := k8sClient.List(ctx, podList, listOptions)
				if err != nil {
					return err
				}

				if len(podList.Items) == 0 {
					return fmt.Errorf("no loki pods found")
				}

				for _, pod := range podList.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return fmt.Errorf("pod %s is not running, current phase: %s", pod.Name, pod.Status.Phase)
					}

					ready := false
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							ready = true
							break
						}
					}
					if !ready {
						return fmt.Errorf("pod %s is not ready", pod.Name)
					}
				}

				return nil
			}).WithPolling(5 * time.Second).WithTimeout(10 * time.Minute).Should(Succeed())
		})
	})
})
