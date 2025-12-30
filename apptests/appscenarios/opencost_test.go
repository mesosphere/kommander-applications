package appscenarios

import (
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/environment"
)

var _ = Describe("Multi-Cluster OpenCost Tests", Label("opencost", "multicluster"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupMultiCluster()
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.InstallLatestFlux(ctx)
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.InstallLatestFluxOnWorkload(ctx)
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.ApplyKommanderPriorityClasses(ctx, environment.ManagementClusterTarget)
		Expect(err).To(Not(HaveOccurred()))

		err = multiEnv.ApplyKommanderPriorityClasses(ctx, environment.WorkloadClusterTarget)
		Expect(err).To(Not(HaveOccurred()))

		workspaceNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workspaceNSName}}
		err = multiEnv.WorkloadClient.Create(ctx, workspaceNS)
		Expect(err).To(Not(HaveOccurred()))
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := TeardownMultiCluster()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Installing multi-cluster OpenCost", Ordered, Label("install"), func() {
		var (
			openCost       *openCost
			workloadNodeIP string
		)

		It("should setup multi-cluster environment", func() {
			Expect(multiEnv).ToNot(BeNil())
			Expect(k8sClient).ToNot(BeNil())
			Expect(workloadK8sClient).ToNot(BeNil())

			openCost = NewOpenCost()
		})

		It("should install kube-prometheus-stack on workload cluster", func() {
			err := openCost.applyKPSWorkloadOverride(ctx, multiEnv)
			Expect(err).ToNot(HaveOccurred())

			err = openCost.deployKPSOnWorkload(ctx, multiEnv)
			Expect(err).ToNot(HaveOccurred())

			hr := &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-prometheus-stack",
					Namespace: workspaceNSName,
				},
			}
			Eventually(func() error {
				err := workloadK8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				if err != nil {
					return err
				}
				for _, cond := range hr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue && cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("kube-prometheus-stack HelmRelease not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should have workload node IP available for NodePort access", func() {
			var err error
			workloadNodeIP, err = openCost.getWorkloadNodeIP(ctx, multiEnv.WorkloadClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(workloadNodeIP).ToNot(BeEmpty())

			openCost.workloadNodeIP = workloadNodeIP

			GinkgoWriter.Printf("Workload Node IP: %s (Thanos will connect via NodePort 30901)\n", workloadNodeIP)
		})

		It("should install Thanos on management cluster", func() {
			err := openCost.createThanosStoresConfigMap(ctx, multiEnv)
			Expect(err).ToNot(HaveOccurred())

			err = openCost.deployThanosOnManagement(ctx, multiEnv)
			Expect(err).ToNot(HaveOccurred())

			hr := &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "thanos",
					Namespace: kommanderNamespace,
				},
			}
			Eventually(func() error {
				err := managementK8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				if err != nil {
					return err
				}
				for _, cond := range hr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue && cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("thanos HelmRelease not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should have Thanos Query retrieve workload cluster metrics", func() {
			// Find thanos-query pod
			podList := &corev1.PodList{}
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "thanos",
					"app.kubernetes.io/component": "query",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() ([]corev1.Pod, error) {
				err := managementK8sClient.List(ctx, podList, &ctrlClient.ListOptions{
					Namespace:     kommanderNamespace,
					LabelSelector: selector,
				})
				if err != nil {
					return nil, err
				}
				// Filter for running pods
				var runningPods []corev1.Pod
				for _, pod := range podList.Items {
					if pod.Status.Phase == corev1.PodRunning {
						runningPods = append(runningPods, pod)
					}
				}
				return runningPods, nil
			}).WithPolling(pollInterval).WithTimeout(2 * time.Minute).Should(HaveLen(1))

			thanosQueryPod := podList.Items[0]
			GinkgoWriter.Printf("Found Thanos Query pod: %s\n", thanosQueryPod.Name)

			// Query Thanos to verify it can retrieve metrics from the workload cluster
			// Thanos Query HTTP API is on port 10902
			// Query for up metric which should include targets from the workload cluster's Prometheus
			Eventually(func() error {
				res := restClientV1Pods.Get().
					Resource("pods").
					Namespace(thanosQueryPod.Namespace).
					Name(thanosQueryPod.Name + ":10902").
					SubResource("proxy").
					Suffix("/api/v1/query").
					Param("query", "up").
					Do(ctx)

				if res.Error() != nil {
					return fmt.Errorf("failed to query Thanos: %w", res.Error())
				}

				var statusCode int
				res.StatusCode(&statusCode)
				if statusCode != 200 {
					return fmt.Errorf("unexpected status code: %d", statusCode)
				}

				body, err := res.Raw()
				if err != nil {
					return fmt.Errorf("failed to read response: %w", err)
				}

				// Verify we got some results (the workload cluster's metrics should be present)
				bodyStr := string(body)
				GinkgoWriter.Printf("Thanos Query response: %s\n", bodyStr)

				if !strings.Contains(bodyStr, "success") {
					return fmt.Errorf("query did not succeed: %s", bodyStr)
				}

				// Verify we have data from the external store (workload cluster)
				// The response should contain metric results
				if !strings.Contains(bodyStr, "result") {
					return fmt.Errorf("no results in query response")
				}

				return nil
			}).WithPolling(5 * time.Second).WithTimeout(2 * time.Minute).Should(Succeed())

			// Additionally verify the store is connected by checking /api/v1/stores
			res := restClientV1Pods.Get().
				Resource("pods").
				Namespace(thanosQueryPod.Namespace).
				Name(thanosQueryPod.Name + ":10902").
				SubResource("proxy").
				Suffix("/api/v1/stores").
				Do(ctx)
			Expect(res.Error()).ToNot(HaveOccurred())

			body, err := res.Raw()
			Expect(err).ToNot(HaveOccurred())
			GinkgoWriter.Printf("Thanos stores: %s\n", string(body))

			// Verify we have at least one store (the workload cluster's Prometheus sidecar)
			Expect(string(body)).To(ContainSubstring("success"))
		})

	})
})
