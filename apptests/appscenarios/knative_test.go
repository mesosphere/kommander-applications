package appscenarios

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			operatorHr    *fluxhelmv2.HelmRelease
			deploymentHr  *fluxhelmv2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install istio-helm as a prerequisite", func() {
			err := k.InstallIstioHelmDependency(ctx, env)
			Expect(err).To(BeNil())

			// Wait for the istiod HelmRelease (the main control plane component)
			istioIstiodHr := &fluxhelmv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-helm-istiod",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(istioIstiodHr), istioIstiodHr)
				if err != nil {
					return err
				}

				for _, cond := range istioIstiodHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("istio-helm-istiod helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should install knative successfully with default config", func() {
			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			operatorHr = &fluxhelmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2.HelmReleaseKind,
					APIVersion: fluxhelmv2.GroupVersion.Version,
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

			deploymentHr = &fluxhelmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2.HelmReleaseKind,
					APIVersion: fluxhelmv2.GroupVersion.Version,
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

		It("should not have any container images using digest references", func() {
			// Wait for jobs to appear in both namespaces before scanning.
			// The operator creates storage-version-migration and cleanup jobs
			// shortly after the CRs are applied. Eventing jobs can be cleaned
			// up quickly, so we poll early to catch them before they disappear.
			for _, ns := range []string{"knative-serving", "knative-eventing"} {
				Eventually(func() int {
					jobList := &batchv1.JobList{}
					if err := k8sClient.List(ctx, jobList, &ctrlClient.ListOptions{Namespace: ns}); err != nil {
						return 0
					}
					return len(jobList.Items)
				}).WithPolling(pollInterval).WithTimeout(3 * time.Minute).Should(
					BeNumerically(">", 0),
					fmt.Sprintf("expected at least 1 job in namespace %s", ns),
				)
			}

			assertNoDigestImages(ctx, "knative-serving")
			assertNoDigestImages(ctx, "knative-eventing")
		})

		It("should have deployments running in knative namespaces", func() {
			// Check knative-serving deployments
			servingSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "knative-serving",
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
					"app.kubernetes.io/name": "knative-eventing",
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

		It("should create the knative-ingress-gateway in knative-serving (NCN-105488)", func() {
			gw := &unstructured.Unstructured{}
			gw.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "networking.istio.io",
				Version: "v1beta1",
				Kind:    "Gateway",
			})

			Eventually(func() error {
				return k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Name:      "knative-ingress-gateway",
					Namespace: "knative-serving",
				}, gw)
			}).WithPolling(pollInterval).WithTimeout(3 * time.Minute).Should(Succeed(),
				"knative-ingress-gateway Gateway must exist in knative-serving after install (regression: NCN-105488)")

			// Verify the gateway has the expected server ports configured
			servers, found, err := unstructured.NestedSlice(gw.Object, "spec", "servers")
			Expect(err).To(BeNil())
			Expect(found).To(BeTrue(), "gateway spec.servers should be present")
			Expect(servers).NotTo(BeEmpty(), "gateway should have at least one server entry")
		})

		It("should use a tagged image for queue-proxy sidecar when deploying a Knative Service", func() {
			assertKnativeServiceQueueProxy(ctx)
		})
	})

	Describe("Knative Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			operatorHr   *fluxhelmv2.HelmRelease
			deploymentHr *fluxhelmv2.HelmRelease
		)

		It("should install istio-helm as a prerequisite", func() {
			err := k.InstallIstioHelmDependency(ctx, env)
			Expect(err).To(BeNil())

			istioIstiodHr := &fluxhelmv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-helm-istiod",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(istioIstiodHr), istioIstiodHr)
				if err != nil {
					return err
				}

				for _, cond := range istioIstiodHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("istio-helm-istiod helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should install the previous version successfully", func() {
			err := k.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			operatorHr = &fluxhelmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2.HelmReleaseKind,
					APIVersion: fluxhelmv2.GroupVersion.Version,
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

			deploymentHr = &fluxhelmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2.HelmReleaseKind,
					APIVersion: fluxhelmv2.GroupVersion.Version,
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

			Eventually(func() (*fluxhelmv2.HelmRelease, error) {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(operatorHr), operatorHr)
				return operatorHr, err
			}, "5m", pollInterval).Should(And(
				HaveField("Status.ObservedGeneration", BeNumerically(">=", existingOperatorGeneration)),
				HaveField("Status.Conditions", ContainElement(And(
					HaveField("Type", Equal(apimeta.ReadyCondition)),
					HaveField("Status", Equal(metav1.ConditionTrue)))),
				),
			))

			Eventually(func() (*fluxhelmv2.HelmRelease, error) {
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

		It("should not have any container images using digest references after upgrade", func() {
			assertNoDigestImages(ctx, "knative-serving")
			assertNoDigestImages(ctx, "knative-eventing")
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

			istioIstiodHr := &fluxhelmv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-helm-istiod",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(istioIstiodHr), istioIstiodHr)
				if err != nil {
					return err
				}

				for _, cond := range istioIstiodHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("istio-helm-istiod helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should install knative successfully", func() {
			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			operatorHr := &fluxhelmv2.HelmRelease{
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

			deploymentHr := &fluxhelmv2.HelmRelease{
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

		It("should verify eventing-webhook has 3 replicas running with pod anti-affinity", func() {
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

			Expect(webhookDeployment.Spec.Replicas).NotTo(BeNil())
			Expect(*webhookDeployment.Spec.Replicas).To(BeNumerically(">", 0),
				"Deployment replicas should not be 0, as PDB with 0 replicas provides no protection")

			affinity := webhookDeployment.Spec.Template.Spec.Affinity
			Expect(affinity).NotTo(BeNil(), "eventing-webhook deployment should have affinity configured")
			Expect(affinity.PodAntiAffinity).NotTo(BeNil(), "eventing-webhook should have podAntiAffinity to spread replicas across nodes")

			preferred := affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
			Expect(preferred).NotTo(BeEmpty(), "eventing-webhook should have preferred pod anti-affinity rules")

			foundHostnameRule := false
			for _, term := range preferred {
				sel := term.PodAffinityTerm.LabelSelector
				if sel == nil {
					continue
				}
				if sel.MatchLabels["app"] == "eventing-webhook" &&
					term.PodAffinityTerm.TopologyKey == "kubernetes.io/hostname" {
					foundHostnameRule = true
					break
				}
			}
			Expect(foundHostnameRule).To(BeTrue(),
				"eventing-webhook should have a preferred anti-affinity rule spreading pods across nodes (topologyKey: kubernetes.io/hostname)")
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

// assertNoDigestImages verifies that no running pods, deployment specs, job
// specs, or image-bearing environment variables in the given namespace use
// digest-based image references (@sha256:...). The Knative operator ships with
// digest references by default; our cm.yaml registry overrides replace them
// with tagged versions. A digest reference at runtime means an override was
// missed and the image will not be available in airgapped environments.
func assertNoDigestImages(ctx context.Context, namespace string) {
	GinkgoHelper()

	var digestViolations []string

	// Check all running pods
	podList := &corev1.PodList{}
	err := k8sClient.List(ctx, podList, &ctrlClient.ListOptions{
		Namespace: namespace,
	})
	Expect(err).To(BeNil())

	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if strings.Contains(cs.Image, "@sha256:") {
				digestViolations = append(digestViolations,
					fmt.Sprintf("pod/%s container=%s image=%s", pod.Name, cs.Name, cs.Image))
			}
		}
		for _, cs := range pod.Status.InitContainerStatuses {
			if strings.Contains(cs.Image, "@sha256:") {
				digestViolations = append(digestViolations,
					fmt.Sprintf("pod/%s init-container=%s image=%s", pod.Name, cs.Name, cs.Image))
			}
		}
	}

	// Check deployment pod template specs and env vars
	deploymentList := &appsv1.DeploymentList{}
	err = k8sClient.List(ctx, deploymentList, &ctrlClient.ListOptions{
		Namespace: namespace,
	})
	Expect(err).To(BeNil())

	for _, deploy := range deploymentList.Items {
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if strings.Contains(c.Image, "@sha256:") {
				digestViolations = append(digestViolations,
					fmt.Sprintf("deployment/%s container=%s image=%s", deploy.Name, c.Name, c.Image))
			}
			// Env vars like QUEUE_SIDECAR_IMAGE, APISERVER_RA_IMAGE, DISPATCHER_IMAGE
			// carry image refs used to spawn pods at runtime. A digest here means
			// the override was missed and any workload triggering that image will
			// fail in airgapped.
			for _, ev := range c.Env {
				if strings.HasSuffix(ev.Name, "_IMAGE") && strings.Contains(ev.Value, "@sha256:") {
					digestViolations = append(digestViolations,
						fmt.Sprintf("deployment/%s container=%s env=%s value=%s",
							deploy.Name, c.Name, ev.Name, ev.Value))
				}
			}
		}
		for _, c := range deploy.Spec.Template.Spec.InitContainers {
			if strings.Contains(c.Image, "@sha256:") {
				digestViolations = append(digestViolations,
					fmt.Sprintf("deployment/%s init-container=%s image=%s", deploy.Name, c.Name, c.Image))
			}
		}
	}

	// Check job pod template specs (storage-version-migration, cleanup jobs)
	jobList := &batchv1.JobList{}
	err = k8sClient.List(ctx, jobList, &ctrlClient.ListOptions{
		Namespace: namespace,
	})
	Expect(err).To(BeNil())

	GinkgoWriter.Printf("[digest-check] found %d job(s) in namespace %s\n", len(jobList.Items), namespace)
	for _, job := range jobList.Items {
		for _, c := range job.Spec.Template.Spec.Containers {
			GinkgoWriter.Printf("[digest-check]   job/%s container=%s image=%s\n", job.Name, c.Name, c.Image)
			if strings.Contains(c.Image, "@sha256:") {
				digestViolations = append(digestViolations,
					fmt.Sprintf("job/%s container=%s image=%s", job.Name, c.Name, c.Image))
			}
		}
		for _, c := range job.Spec.Template.Spec.InitContainers {
			GinkgoWriter.Printf("[digest-check]   job/%s init-container=%s image=%s\n", job.Name, c.Name, c.Image)
			if strings.Contains(c.Image, "@sha256:") {
				digestViolations = append(digestViolations,
					fmt.Sprintf("job/%s init-container=%s image=%s", job.Name, c.Name, c.Image))
			}
		}
	}

	// Check ConfigMap data values for digest references. The Knative operator
	// stores integration/source/sink images in ConfigMaps (e.g.
	// eventing-integrations-images, eventing-transformations-images) and env
	// vars reference them via valueFrom.configMapKeyRef. The operator's
	// registry overrides should replace these digests with tagged versions.
	cmList := &corev1.ConfigMapList{}
	err = k8sClient.List(ctx, cmList, &ctrlClient.ListOptions{
		Namespace: namespace,
	})
	Expect(err).To(BeNil())

	for _, cm := range cmList.Items {
		for key, val := range cm.Data {
			if strings.Contains(val, "@sha256:") {
				GinkgoWriter.Printf("[digest-check]   configmap/%s key=%s value=%s\n", cm.Name, key, val)
				digestViolations = append(digestViolations,
					fmt.Sprintf("configmap/%s key=%s value=%s", cm.Name, key, val))
			}
		}
	}

	if len(digestViolations) > 0 {
		GinkgoWriter.Printf("Found %d image(s) using digest references in namespace %s:\n", len(digestViolations), namespace)
		for _, v := range digestViolations {
			GinkgoWriter.Printf("  - %s\n", v)
		}
	}

	Expect(digestViolations).To(BeEmpty(),
		fmt.Sprintf("Found %d container image(s) or env var(s) in namespace %s using digest (@sha256:) references instead of tags. "+
			"These images are missing registry overrides in cm.yaml and will fail in airgapped installs. "+
			"Run hack/knative/extract-images.py to regenerate overrides.",
			len(digestViolations), namespace))
}

// assertKnativeServiceQueueProxy deploys a minimal Knative Service and verifies
// that the queue-proxy sidecar injected by the controller uses a tagged image,
// not a digest reference. This is the only way to exercise the QUEUE_SIDECAR_IMAGE
// override end-to-end, since the sidecar is never created during a bare install.
func assertKnativeServiceQueueProxy(ctx context.Context) {
	GinkgoHelper()

	ksvc := &unstructured.Unstructured{}
	ksvc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "serving.knative.dev",
		Version: "v1",
		Kind:    "Service",
	})
	ksvc.SetName("image-override-test")
	ksvc.SetNamespace("knative-serving")
	err := unstructured.SetNestedMap(ksvc.Object, map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"image": "gcr.io/knative-samples/helloworld-go",
						"env": []interface{}{
							map[string]interface{}{
								"name":  "TARGET",
								"value": "image-override-test",
							},
						},
					},
				},
			},
		},
	}, "spec")
	Expect(err).To(BeNil())

	// The serving webhook may not be accepting connections yet; retry until
	// the admission webhook is reachable and the create succeeds.
	Eventually(func() error {
		return k8sClient.Create(ctx, ksvc)
	}).WithPolling(pollInterval).WithTimeout(2 * time.Minute).Should(Succeed())

	// Wait for the ksvc to create pods with the queue-proxy sidecar
	var queueProxyImage string
	Eventually(func() error {
		podList := &corev1.PodList{}
		err := k8sClient.List(ctx, podList, &ctrlClient.ListOptions{
			Namespace: "knative-serving",
		})
		if err != nil {
			return err
		}

		for _, pod := range podList.Items {
			if !strings.HasPrefix(pod.Name, "image-override-test-") {
				continue
			}
			for _, c := range pod.Spec.Containers {
				if c.Name == "queue-proxy" {
					queueProxyImage = c.Image
					return nil
				}
			}
		}
		return fmt.Errorf("no queue-proxy sidecar found on image-override-test pods yet")
	}).WithPolling(pollInterval).WithTimeout(3 * time.Minute).Should(Succeed())

	GinkgoWriter.Printf("queue-proxy sidecar image: %s\n", queueProxyImage)
	Expect(queueProxyImage).NotTo(ContainSubstring("@sha256:"),
		fmt.Sprintf("queue-proxy sidecar is using a digest reference (%s) instead of a tagged image. "+
			"The QUEUE_SIDECAR_IMAGE override in cm.yaml is missing or incorrect.", queueProxyImage))

	// Clean up
	err = k8sClient.Delete(ctx, ksvc)
	Expect(err).To(BeNil())
}
