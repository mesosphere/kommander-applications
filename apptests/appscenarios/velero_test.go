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
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Velero Local Backup Tests", Label("velero"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		// Setup the block storage for rook ceph - needed for local backup
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

	Describe("Installing Velero", Ordered, Label("install"), func() {

		var (
			v              velero
			rc             rookCeph
			hr             *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install rook ceph successfully as a prerequisite", func() {
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

		It("should create storage cluster", func() {
			err := rc.CreateBucketPreReqs(ctx, env)
			Expect(err).To(BeNil())

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

		It("should install velero successfully with default config", func() {
			v = velero{}
			err := v.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.Name(),
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
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})

		It("should have a PriorityClass configured", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": v.Name(),
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
				Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal(dkpCriticalPriority))
			}
		})

		It("should create an nginx app for testing", func() {
			err := v.CreateNginxApp(ctx, env)
			Expect(err).To(BeNil())

			// Check the status of the nginx deployment
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			// Check the status of the nginx deployment
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return err
				}

				if deploymentList.Items[0].Status.ReadyReplicas == 1 {
					return nil
				}
				return fmt.Errorf("nginx deployment not ready yet")
			})
		})

		It("should back up the nginx app", func() {
			err := v.Backup(ctx, env, "nginx-backup")
			Expect(err).To(BeNil())

			// Check the status of the backup
			backup := &unstructured.Unstructured{}
			backup.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "velero.io",
				Kind:    "Backup",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: kommanderNamespace,
					Name:      "nginx-backup",
				}, backup)
				if err != nil {
					return err
				}

				phase, _, _ := unstructured.NestedString(backup.Object, "status", "phase")
				if phase == "Completed" {
					return nil
				}

				return fmt.Errorf("backup not complete")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should delete the nginx app to simulate a disaster", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			err = k8sClient.Delete(ctx, &deploymentList.Items[0])
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return nil
				}
				return fmt.Errorf("nginx deployment not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			svc := &corev1.Service{}
			err = k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Namespace: deploymentList.Items[0].Namespace,
				Name:      "nginx",
			}, svc)
			Expect(err).To(BeNil())

			err = k8sClient.Delete(ctx, svc)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      "nginx",
				}, svc)
				if err != nil {
					return nil
				}
				return fmt.Errorf("nginx service not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			ns := &corev1.Namespace{}
			err = k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Name: deploymentList.Items[0].Namespace,
			}, ns)
			Expect(err).To(BeNil())

			err = k8sClient.Delete(ctx, ns)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Name: deploymentList.Items[0].Namespace,
				}, ns)
				if err != nil {
					return nil
				}
				return fmt.Errorf("namespace not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should restore the nginx app", func() {
			err := v.Restore(ctx, env, "nginx-backup", "nginx-backup-restore")
			Expect(err).To(BeNil())

			restore := &unstructured.Unstructured{}
			restore.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "velero.io",
				Kind:    "Restore",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: kommanderNamespace,
					Name:      "nginx-backup-restore",
				}, restore)
				if err != nil {
					return err
				}

				phase, _, _ := unstructured.NestedString(restore.Object, "status", "phase")
				if phase == "Completed" {
					return nil
				}

				return fmt.Errorf("restore not complete")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return err
				}

				if deploymentList.Items[0].Status.ReadyReplicas == 1 {
					return nil
				}
				return fmt.Errorf("nginx deployment not ready yet")
			})
		})
	})

	Describe("Upgrading velero", Ordered, Label("upgrade"), func() {
		var (
			v              velero
			rc             rookCeph
			hr             *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install rook ceph successfully as a prerequisite", func() {
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

		It("should install the previous version of velero successfully", func() {
			v = velero{}
			err := v.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.Name(),
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

		It("should create an nginx app for testing", func() {
			err := v.CreateNginxApp(ctx, env)
			Expect(err).To(BeNil())

			// Check the status of the nginx deployment
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			// Check the status of the nginx deployment
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return err
				}

				if deploymentList.Items[0].Status.ReadyReplicas == 1 {
					return nil
				}
				return fmt.Errorf("nginx deployment not ready yet")
			})
		})

		It("should back up the nginx app", func() {
			err := v.Backup(ctx, env, "nginx-backup")
			Expect(err).To(BeNil())

			// Check the status of the backup
			backup := &unstructured.Unstructured{}
			backup.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "velero.io",
				Kind:    "Backup",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: kommanderNamespace,
					Name:      "nginx-backup",
				}, backup)
				if err != nil {
					return err
				}

				phase, _, _ := unstructured.NestedString(backup.Object, "status", "phase")
				if phase == "Completed" {
					return nil
				}

				return fmt.Errorf("backup not complete")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should delete the nginx app to simulate a disaster", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			err = k8sClient.Delete(ctx, &deploymentList.Items[0])
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return nil
				}
				return fmt.Errorf("nginx deployment not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			svc := &corev1.Service{}
			err = k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Namespace: deploymentList.Items[0].Namespace,
				Name:      "nginx",
			}, svc)
			Expect(err).To(BeNil())

			err = k8sClient.Delete(ctx, svc)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      "nginx",
				}, svc)
				if err != nil {
					return nil
				}
				return fmt.Errorf("nginx service not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			ns := &corev1.Namespace{}
			err = k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Name: deploymentList.Items[0].Namespace,
			}, ns)
			Expect(err).To(BeNil())

			err = k8sClient.Delete(ctx, ns)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Name: deploymentList.Items[0].Namespace,
				}, ns)
				if err != nil {
					return nil
				}
				return fmt.Errorf("namespace not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should restore the nginx app", func() {
			err := v.Restore(ctx, env, "nginx-backup", "nginx-backup-pre-upgrade-restore")
			Expect(err).To(BeNil())

			// Check the status of the restore
			restore := &unstructured.Unstructured{}
			restore.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "velero.io",
				Kind:    "Restore",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: kommanderNamespace,
					Name:      "nginx-backup-pre-upgrade-restore",
				}, restore)
				if err != nil {
					return err
				}

				phase, _, _ := unstructured.NestedString(restore.Object, "status", "phase")
				if phase == "Completed" {
					return nil
				}

				return fmt.Errorf("restore not complete")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			// Check the status of the nginx deployment
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			// Check the status of the nginx deployment
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return err
				}

				if deploymentList.Items[0].Status.ReadyReplicas == 1 {
					return nil
				}
				return fmt.Errorf("nginx deployment not ready yet")
			})
		})

		It("should upgrade velero successfully", func() {
			err := v.Upgrade(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      v.Name(),
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

		It("should create an nginx app for testing", func() {
			err := v.CreateNginxApp(ctx, env)
			Expect(err).To(BeNil())

			// Check the status of the nginx deployment
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			// Check the status of the nginx deployment
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return err
				}

				if deploymentList.Items[0].Status.ReadyReplicas == 1 {
					return nil
				}
				return fmt.Errorf("nginx deployment not ready yet")
			})
		})

		It("should back up the nginx app", func() {
			err := v.Backup(ctx, env, "nginx-backup")
			Expect(err).To(BeNil())

			// Check the status of the backup
			backup := &unstructured.Unstructured{}
			backup.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "velero.io",
				Kind:    "Backup",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: kommanderNamespace,
					Name:      "nginx-backup-post-upgrade",
				}, backup)
				if err != nil {
					return err
				}

				phase, _, _ := unstructured.NestedString(backup.Object, "status", "phase")
				if phase == "Completed" {
					return nil
				}

				return fmt.Errorf("backup not complete")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should delete the nginx app", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			err = k8sClient.Delete(ctx, &deploymentList.Items[0])
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return nil
				}
				return fmt.Errorf("nginx deployment not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			svc := &corev1.Service{}
			err = k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Namespace: deploymentList.Items[0].Namespace,
				Name:      "nginx",
			}, svc)
			Expect(err).To(BeNil())

			err = k8sClient.Delete(ctx, svc)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      "nginx",
				}, svc)
				if err != nil {
					return nil
				}
				return fmt.Errorf("nginx service not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			ns := &corev1.Namespace{}
			err = k8sClient.Get(ctx, ctrlClient.ObjectKey{
				Name: deploymentList.Items[0].Namespace,
			}, ns)
			Expect(err).To(BeNil())

			err = k8sClient.Delete(ctx, ns)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Name: deploymentList.Items[0].Namespace,
				}, ns)
				if err != nil {
					return nil
				}
				return fmt.Errorf("namespace not deleted yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should restore the nginx app", func() {
			err := v.Restore(ctx, env, "nginx-backup-post-upgrade", "nginx-backup-post-upgrade-restore")
			Expect(err).To(BeNil())

			// Check the status of the restore
			restore := &unstructured.Unstructured{}
			restore.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "velero.io",
				Kind:    "Restore",
				Version: "v1",
			})

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: kommanderNamespace,
					Name:      "nginx-backup-restore",
				}, restore)
				if err != nil {
					return err
				}

				phase, _, _ := unstructured.NestedString(restore.Object, "status", "phase")
				if phase == "Completed" {
					return nil
				}

				return fmt.Errorf("restore not complete")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			// Check the status of the nginx deployment
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
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

			// Check the status of the nginx deployment
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: deploymentList.Items[0].Namespace,
					Name:      deploymentList.Items[0].Name,
				}, &deploymentList.Items[0])
				if err != nil {
					return err
				}

				if deploymentList.Items[0].Status.ReadyReplicas == 1 {
					return nil
				}
				return fmt.Errorf("nginx deployment not ready yet")
			})
		})
	})
})
