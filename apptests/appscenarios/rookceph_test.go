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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Rook Ceph Tests", Label("rook-ceph", "rook-ceph-cluster"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		// Setup the block storage for rook ceph
		rc := rookCeph{}
		err = rc.CreateLoopbackDevicesKind(ctx, env)
		Expect(err).To(BeNil())

		err = env.RunScriptOnAllNode(ctx, "/hack/scripts/loopback-storage-creator.sh")
		Expect(err).To(BeNil())

		err = rc.ApplyPersistentVolumeCreator(ctx, env)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())
	})

	AfterEach(OncePerOrdered, func() {
		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Installing Rook Ceph", Ordered, Label("install"), func() {
		var (
			rc             rookCeph
			hr             *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install successfully with default config", func() {
			rc = rookCeph{}
			err := rc.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rc.Name(),
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

		It("should have a PriorityClass configured", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": rc.Name(),
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
				Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal(systemClusterCriticalPriority))
			}
		})

		It("should create storage cluster", func() {
			err := rc.CreateBucketPreReqs(ctx, env)
			Expect(err).To(BeNil())

			// Wait for the pre-install job to complete
			job := &unstructured.Unstructured{}
			job.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "batch",
				Kind:    "Job",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx,
					ctrlClient.ObjectKey{
						Namespace: kommanderNamespace,
						Name:      "dkp-ceph-prereq-job",
					}, job)
				if err != nil {
					return err
				}

				conditions, _, _ := unstructured.NestedSlice(job.Object, "status", "conditions")
				for _, c := range conditions {
					condition := c.(map[string]interface{})
					if condition["type"] == "Complete" && condition["status"] == "True" {
						return nil
					}
				}
				return fmt.Errorf("job not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			err = rc.CreateBuckets(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rook-ceph-cluster",
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

		It("should create storage buckets", func() {
			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "object-bucket-claims",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
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

		It("should have access to the dashboard", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":          "rook-ceph-mgr",
					"mgr_role":     "active",
					"rook_cluster": "kommander",
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

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":8443").SubResource("proxy").Suffix("").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("Ceph"))
		})
	})

	Describe("Upgrading Rook Ceph", Ordered, Label("upgrade"), func() {
		var (
			rc rookCeph
			hr *fluxhelmv2beta2.HelmRelease
		)

		It("should install the previous version successfully", func() {
			rc = rookCeph{}
			err := rc.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rc.Name(),
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

		It("should create storage cluster", func() {
			err := rc.CreateBucketPreReqsPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			// Wait for the pre-install job to complete
			job := &unstructured.Unstructured{}
			job.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "batch",
				Kind:    "Job",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx,
					ctrlClient.ObjectKey{
						Namespace: kommanderNamespace,
						Name:      "dkp-ceph-prereq-job",
					}, job)
				if err != nil {
					return err
				}

				conditions, _, _ := unstructured.NestedSlice(job.Object, "status", "conditions")
				for _, c := range conditions {
					condition := c.(map[string]interface{})
					if condition["type"] == "Complete" && condition["status"] == "True" {
						return nil
					}
				}
				return fmt.Errorf("job not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			err = rc.CreateBucketsPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rook-ceph-cluster",
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

		It("should create storage buckets", func() {
			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "object-bucket-claims",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
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

		It("should have access to the dashboard", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":          "rook-ceph-mgr",
					"mgr_role":     "active",
					"rook_cluster": "kommander",
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

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":8443").SubResource("proxy").Suffix("").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("Ceph"))
		})

		It("should remove previous job", func() {
			// Delete the previous job if it exists
			clientset, err := kubernetes.NewForConfig(env.K8sClient.Config())
			Expect(err).To(BeNil())

			err = clientset.BatchV1().Jobs(kommanderNamespace).Delete(ctx, "dkp-ceph-prereq-job", metav1.DeleteOptions{})
			Expect(err).To(BeNil())
		})

		It("should upgrade rook ceph successfully", func() {
			err := rc.Upgrade(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      rc.Name(),
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

		It("should reconcile storage cluster after upgrade", func() {
			err := rc.CreateBucketPreReqs(ctx, env)
			Expect(err).To(BeNil())

			// Wait for the pre-install job to complete
			job := &unstructured.Unstructured{}
			job.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "batch",
				Kind:    "Job",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx,
					ctrlClient.ObjectKey{
						Namespace: kommanderNamespace,
						Name:      "dkp-ceph-prereq-job",
					}, job)
				if err != nil {
					return err
				}

				conditions, _, _ := unstructured.NestedSlice(job.Object, "status", "conditions")
				for _, c := range conditions {
					condition := c.(map[string]interface{})
					if condition["type"] == "Complete" && condition["status"] == "True" {
						return nil
					}
				}
				return fmt.Errorf("job not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			err = rc.CreateBuckets(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rook-ceph-cluster",
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

		It("should reconcile storage buckets after upgrade", func() {
			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "object-bucket-claims",
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
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

		It("should have access to the dashboard after upgrade", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":          "rook-ceph-mgr",
					"mgr_role":     "active",
					"rook_cluster": "kommander",
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

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":8443").SubResource("proxy").Suffix("").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("Ceph"))
		})
	})
})
