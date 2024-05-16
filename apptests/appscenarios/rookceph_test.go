package appscenarios

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Rook Ceph Tests", Label("rook-ceph"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		// create block storage in kind for rook ceph
		rc := rookCeph{}
		err = rc.CreateLoopbackDevicesKind(ctx, env)
		Expect(err).To(BeNil())

		err = env.RunScriptAllNode(ctx, "/hack/scripts/loopback-storage-creator.sh")
		Expect(err).To(BeNil())

		err = rc.ApplyPersistentVolumeCreator(ctx, env)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())
	})

	AfterEach(OncePerOrdered, func() {
		//err := env.Destroy(ctx)
		//Expect(err).ToNot(HaveOccurred())
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
			// Check the status of the Rook Ceph cluster
			err := rc.CreateBuckets(ctx, env)
			Expect(err).To(BeNil())

			// Check the HelmRelease for rook-ceph-cluster
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
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

			// Check the status of the ObjectBucketClaims
			Eventually(func() error {
				err := checkOBClaim("dkp-insights")
				if err != nil {
					return err
				}

				err = checkOBClaim("dkp-loki")
				if err != nil {
					return err
				}

				return checkOBClaim("dkp-velero")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should be responding to requests for the dashboard on port 8443", func() {
			res := restClientV1Services.Get().Resource("service").Namespace(kommanderNamespace).Name("rook-ceph-mgr-dashboard:8443").SubResource("proxy").Suffix("").Do(ctx)
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

		It("should create storage buckets", func() {
			// Check the status of the Rook Ceph cluster
			err := rc.CreateBuckets(ctx, env)
			Expect(err).To(BeNil())

			// Check the HelmRelease for rook-ceph-cluster
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

			// Check the status of the ObjectBucketClaims
			Eventually(func() error {
				err := checkOBClaim("dkp-insights")
				if err != nil {
					return err
				}

				err = checkOBClaim("dkp-loki")
				if err != nil {
					return err
				}

				return checkOBClaim("dkp-velero")
			}).WithPolling(pollInterval).WithTimeout(20 * time.Minute).Should(Succeed())
		})

		It("should be responding to requests for the dashboard on port 8443", func() {
			res := restClientV1Services.Get().Resource("service").Namespace(kommanderNamespace).Name("rook-ceph-mgr-dashboard:8443").SubResource("proxy").Suffix("").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))

			body, err := res.Raw()
			Expect(err).To(BeNil())
			Expect(string(body)).To(ContainSubstring("Ceph"))
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

		It("should create storage buckets", func() {
			// Check the status of the Rook Ceph cluster
			err := rc.CreateBuckets(ctx, env)
			Expect(err).To(BeNil())

			// Check the HelmRelease for rook-ceph-cluster
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

			// Check the status of the ObjectBucketClaims
			Eventually(func() error {
				err := checkOBClaim("dkp-insights")
				if err != nil {
					return err
				}

				err = checkOBClaim("dkp-loki")
				if err != nil {
					return err
				}

				return checkOBClaim("dkp-velero")
			}).WithPolling(pollInterval).WithTimeout(20 * time.Minute).Should(Succeed())
		})

		It("should be responding to requests for the dashboard on port 8443", func() {
			res := restClientV1Services.Get().Resource("service").Namespace(kommanderNamespace).Name("rook-ceph-mgr-dashboard:8443").SubResource("proxy").Suffix("").Do(ctx)
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

func checkOBClaim(bucketName string) error {
	objectbucketclaim := &unstructured.Unstructured{}
	objectbucketclaim.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "objectbucket.io",
		Kind:    "ObjectBucketClaim",
		Version: "v1alpha1",
	})

	err := k8sClient.Get(ctx,
		ctrlClient.ObjectKey{
			Namespace: kommanderNamespace,
			Name:      bucketName,
		}, objectbucketclaim)
	if err != nil {
		return err
	}

	conditions, _, _ := unstructured.NestedSlice(objectbucketclaim.Object, "status", "phase")
	for _, c := range conditions {
		condition := c.(map[string]interface{})
		if condition["phase"] == "Bound" {
			return nil
		}
	}
	return fmt.Errorf("objectbucketclaim not ready yet")
}
