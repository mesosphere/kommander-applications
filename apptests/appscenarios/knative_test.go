package appscenarios

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Knative Tests", Label("knative"), func() {
	var k *knative

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		k = NewKnative()
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Knative Install Test", Ordered, Label("install"), func() {
		var (
			istioHr       *fluxhelmv2beta2.HelmRelease
			operatorHr    *fluxhelmv2beta2.HelmRelease
			deploymentHr  *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install istio-helm as a prerequisite", func() {
			err := k.InstallIstioHelmDependency(ctx, env)
			Expect(err).To(BeNil())

			istioHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-helm",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(istioHr), istioHr)
				if err != nil {
					return err
				}

				for _, cond := range istioHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("istio-helm helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should install knative successfully with default config", func() {
			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			operatorHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "knative-operator",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(operatorHr), operatorHr)
				if err != nil {
					return err
				}

				for _, cond := range operatorHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("knative-operator helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())

			deploymentHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "knative-deploy",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(deploymentHr), deploymentHr)
				if err != nil {
					return err
				}

				for _, cond := range deploymentHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("knative-deploy helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should have deployments running in knative namespaces", func() {
			// Check knative-serving deployments
			servingSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/part-of": "knative-serving",
				},
			})
			Expect(err).To(BeNil())

			deploymentList = &appsv1.DeploymentList{}
			Eventually(func() error {
				err := k8sClient.List(ctx, deploymentList, &ctrlClient.ListOptions{
					LabelSelector: servingSelector,
					Namespace:     "knative-serving",
				})
				if err != nil {
					return err
				}
				if len(deploymentList.Items) == 0 {
					return fmt.Errorf("no knative-serving deployments found")
				}
				return nil
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			// Check knative-eventing deployments
			eventingSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/part-of": "knative-eventing",
				},
			})
			Expect(err).To(BeNil())

			deploymentList = &appsv1.DeploymentList{}
			Eventually(func() error {
				err := k8sClient.List(ctx, deploymentList, &ctrlClient.ListOptions{
					LabelSelector: eventingSelector,
					Namespace:     "knative-eventing",
				})
				if err != nil {
					return err
				}
				if len(deploymentList.Items) == 0 {
					return fmt.Errorf("no knative-eventing deployments found")
				}
				return nil
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})
	})

	Describe("Knative Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			operatorHr   *fluxhelmv2beta2.HelmRelease
			deploymentHr *fluxhelmv2beta2.HelmRelease
		)

		It("should install istio-helm as a prerequisite", func() {
			err := k.InstallIstioHelmDependency(ctx, env)
			Expect(err).To(BeNil())

			istioHr := &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-helm",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(istioHr), istioHr)
				if err != nil {
					return err
				}

				for _, cond := range istioHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("istio-helm helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should install the previous version successfully", func() {
			err := k.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			operatorHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "knative-operator",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(operatorHr), operatorHr)
				if err != nil {
					return err
				}

				for _, cond := range operatorHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("knative-operator helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())

			deploymentHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "knative-deploy",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(deploymentHr), deploymentHr)
				if err != nil {
					return err
				}

				for _, cond := range deploymentHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("knative-deploy helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should upgrade knative successfully", func() {
			Expect(k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(operatorHr), operatorHr)).To(Succeed())
			existingOperatorGeneration := operatorHr.Status.ObservedGeneration

			Expect(k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(deploymentHr), deploymentHr)).To(Succeed())
			existingDeploymentGeneration := deploymentHr.Status.ObservedGeneration

			err := k.Upgrade(ctx, env)
			Expect(err).To(BeNil())

			Eventually(func() (*fluxhelmv2beta2.HelmRelease, error) {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(operatorHr), operatorHr)
				return operatorHr, err
			}, "5m", pollInterval).Should(And(
				HaveField("Status.ObservedGeneration", BeNumerically(">=", existingOperatorGeneration)),
				HaveField("Status.Conditions", ContainElement(And(
					HaveField("Type", Equal(apimeta.ReadyCondition)),
					HaveField("Status", Equal(metav1.ConditionTrue)))),
				),
			))

			Eventually(func() (*fluxhelmv2beta2.HelmRelease, error) {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(deploymentHr), deploymentHr)
				return deploymentHr, err
			}, "5m", pollInterval).Should(And(
				HaveField("Status.ObservedGeneration", BeNumerically(">=", existingDeploymentGeneration)),
				HaveField("Status.Conditions", ContainElement(And(
					HaveField("Type", Equal(apimeta.ReadyCondition)),
					HaveField("Status", Equal(metav1.ConditionTrue)))),
				),
			))
		})
	})

	Describe("Knative PDB Drain Resilience Test", Ordered, Label("pdb-drain"), func() {
		var (
			clientset     *kubernetes.Clientset
			workerNode    string
			pdb           *policyv1.PodDisruptionBudget
			webhookPods   *corev1.PodList
		)

		It("should install istio-helm as a prerequisite", func() {
			err := k.InstallIstioHelmDependency(ctx, env)
			Expect(err).To(BeNil())

			istioHr := &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-helm",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(istioHr), istioHr)
				if err != nil {
					return err
				}

				for _, cond := range istioHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("istio-helm helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should install knative successfully", func() {
			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			operatorHr := &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "knative-operator",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(operatorHr), operatorHr)
				if err != nil {
					return err
				}

				for _, cond := range operatorHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("knative-operator helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())

			deploymentHr := &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "knative-deploy",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(deploymentHr), deploymentHr)
				if err != nil {
					return err
				}

				for _, cond := range deploymentHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("knative-deploy helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should verify PDB exists with correct configuration", func() {
			pdb = &policyv1.PodDisruptionBudget{}
			err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Name:      "eventing-webhook",
				Namespace: "knative-eventing",
			}, pdb)
			Expect(err).To(BeNil())
			Expect(pdb.Spec.MaxUnavailable).NotTo(BeNil())
			Expect(pdb.Spec.MaxUnavailable.IntVal).To(Equal(int32(1)))
		})

		It("should verify eventing-webhook has 3 replicas running", func() {
			webhookDeployment := &appsv1.Deployment{}
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Name:      "eventing-webhook",
					Namespace: "knative-eventing",
				}, webhookDeployment)
				if err != nil {
					return err
				}

				if webhookDeployment.Status.ReadyReplicas < 3 {
					return fmt.Errorf("expected 3 ready replicas, got %d", webhookDeployment.Status.ReadyReplicas)
				}
				return nil
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			// Fail early if replicas is 0 - this is the bug we're trying to catch
			Expect(webhookDeployment.Spec.Replicas).NotTo(BeNil())
			Expect(*webhookDeployment.Spec.Replicas).To(BeNumerically(">", 0), 
				"Deployment replicas should not be 0, as PDB with 0 replicas provides no protection")
		})

		It("should get the worker node name", func() {
			var err error
			clientset, err = kubernetes.NewForConfig(env.K8sClient.Config())
			Expect(err).To(BeNil())

			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			Expect(err).To(BeNil())

			// Find the worker node (not the control-plane)
			for _, node := range nodes.Items {
				if _, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]; !isControlPlane {
					workerNode = node.Name
					break
				}
			}
			Expect(workerNode).NotTo(BeEmpty(), "Should find a worker node")
		})

		It("should drain the worker node and respect PDB constraints", func() {
			// Get webhook pods before drain
			webhookPods = &corev1.PodList{}
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "eventing-webhook",
				},
			})
			Expect(err).To(BeNil())

			err = k8sClient.List(ctx, webhookPods, &ctrlClient.ListOptions{
				LabelSelector: selector,
				Namespace:     "knative-eventing",
			})
			Expect(err).To(BeNil())
			initialPodCount := len(webhookPods.Items)
			Expect(initialPodCount).To(Equal(3), "Should have 3 webhook pods before drain")

			// Count pods on worker node
			podsOnWorker := 0
			for _, pod := range webhookPods.Items {
				if pod.Spec.NodeName == workerNode {
					podsOnWorker++
				}
			}
			GinkgoWriter.Printf("Found %d eventing-webhook pods on worker node %s\n", podsOnWorker, workerNode)

			// Cordon the node
			node, err := clientset.CoreV1().Nodes().Get(ctx, workerNode, metav1.GetOptions{})
			Expect(err).To(BeNil())
			node.Spec.Unschedulable = true
			_, err = clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			Expect(err).To(BeNil())

			// Evict pods on the worker node one at a time using the eviction API,
			// which respects PDB constraints. A small delay between evictions gives
			// the scheduler time to reschedule before the next eviction is attempted.
			drainCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			for _, pod := range webhookPods.Items {
				if pod.Spec.NodeName == workerNode {
					eviction := &policyv1.Eviction{
						ObjectMeta: metav1.ObjectMeta{
							Name:      pod.Name,
							Namespace: pod.Namespace,
						},
					}
					evictErr := clientset.PolicyV1().Evictions(pod.Namespace).Evict(drainCtx, eviction)
					if evictErr != nil {
						GinkgoWriter.Printf("Eviction of pod %s returned (expected PDB backpressure): %v\n", pod.Name, evictErr)
					}
					time.Sleep(2 * time.Second)
				}
			}

			// Monitor that minimum availability is maintained throughout the drain.
			// At least (replicas - maxUnavailable) = 3 - 1 = 2 pods must stay ready.
			Consistently(func() int {
				pods := &corev1.PodList{}
				err := k8sClient.List(ctx, pods, &ctrlClient.ListOptions{
					LabelSelector: selector,
					Namespace:     "knative-eventing",
				})
				if err != nil {
					return 0
				}

				availableCount := 0
				for _, pod := range pods.Items {
					if pod.Status.Phase == corev1.PodRunning {
						for _, condition := range pod.Status.Conditions {
							if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
								availableCount++
								break
							}
						}
					}
				}
				return availableCount
			}, "45s", "3s").Should(BeNumerically(">=", 2),
				"PDB should ensure at least 2 pods remain available during drain")

			// Wait for evicted pods to be rescheduled away from the drained node.
			// Timeout here means PDB is blocking evictions or pods cannot reschedule.
			Eventually(func() error {
				pods := &corev1.PodList{}
				err := k8sClient.List(drainCtx, pods, &ctrlClient.ListOptions{
					LabelSelector: selector,
					Namespace:     "knative-eventing",
				})
				if err != nil {
					return err
				}

				runningOffNode := 0
				for _, pod := range pods.Items {
					if pod.Status.Phase == corev1.PodRunning && pod.Spec.NodeName != workerNode {
						runningOffNode++
					}
				}
				if runningOffNode >= 2 {
					return nil
				}
				return fmt.Errorf("drain timed out - only %d pods running off the drained node; PDB may be blocking evictions or pods cannot be rescheduled", runningOffNode)
			}).WithContext(drainCtx).WithPolling(pollInterval).WithTimeout(90 * time.Second).Should(Succeed())

			// Verify final state
			finalPods := &corev1.PodList{}
			err = k8sClient.List(ctx, finalPods, &ctrlClient.ListOptions{
				LabelSelector: selector,
				Namespace:     "knative-eventing",
			})
			Expect(err).To(BeNil())

			runningPods := 0
			for _, pod := range finalPods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPods++
				}
			}
			Expect(runningPods).To(BeNumerically(">=", 2), 
				"Should maintain at least 2 running pods after drain completes")
		})
	})
})



