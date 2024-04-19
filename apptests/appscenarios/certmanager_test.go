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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Installing Cert Manager", Ordered, Label("cert-manager", "install"), func() {

	var (
		cm             certManager
		hr, hrCrds     *fluxhelmv2beta2.HelmRelease
		ns             *corev1.Namespace
		rq             *corev1.ResourceQuota
		deploymentList *appsv1.DeploymentList
	)

	It("should install successfully with default config", func() {
		cm = certManager{}
		err := cm.Install(ctx, env)
		Expect(err).To(BeNil())

		hr = &fluxhelmv2beta2.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       fluxhelmv2beta2.HelmReleaseKind,
				APIVersion: fluxhelmv2beta2.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.Name(),
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

	It("should install crds successfully", func() {
		hrCrds = &fluxhelmv2beta2.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       fluxhelmv2beta2.HelmReleaseKind,
				APIVersion: fluxhelmv2beta2.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-manager-crds",
				Namespace: kommanderNamespace,
			},
		}

		Eventually(func() error {
			err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hrCrds), hrCrds)
			if err != nil {
				return err
			}

			for _, cond := range hrCrds.Status.Conditions {
				if cond.Status == metav1.ConditionTrue &&
					cond.Type == apimeta.ReadyCondition {
					return nil
				}
			}
			return fmt.Errorf("helm release not ready yet")
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should create the cert-manager namespace", func() {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: cm.Name(),
			},
		}

		Eventually(func() error {
			err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(ns), ns)
			if err != nil {
				return err
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should create a ResourceQuota", func() {
		rq = &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-manager-critical-pods",
				Namespace: cm.Name(),
			},
		}

		Eventually(func() error {
			err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(rq), rq)
			if err != nil {
				return err
			}

			Expect(rq.Spec.Hard).To(HaveKey(corev1.ResourcePods))
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should have a PriorityClass configured", func() {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"helm.toolkit.fluxcd.io/name": cm.Name(),
			},
		})
		Expect(err).To(BeNil())
		listOptions := &ctrlClient.ListOptions{
			LabelSelector: selector,
		}
		deploymentList = &appsv1.DeploymentList{}
		err = k8sClient.List(ctx, deploymentList, listOptions)
		Expect(err).To(BeNil())
		Expect(deploymentList.Items).To(HaveLen(3))
		Expect(err).To(BeNil())

		Expect(deploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))
	})

	It("should create the root CA successfully", func() {
		err := cm.InstallRootCA(ctx, env)
		Expect(err).To(BeNil())

		rootCA := &unstructured.Unstructured{}
		rootCA.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cert-manager.io",
			Kind:    "ClusterIssuer",
			Version: "v1",
		})

		Eventually(func() error {
			err := k8sClient.Get(ctx,
				ctrlClient.ObjectKey{
					Name: "kommander-ca",
				}, rootCA)
			if err != nil {
				return err
			}
			conditions, _, _ := unstructured.NestedSlice(rootCA.Object, "status", "conditions")

			for _, c := range conditions {
				condition := c.(map[string]interface{})
				if condition["type"] == "Ready" && condition["status"] == "True" {
					return nil
				}
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should issue a certificate successfully", func() {
		err := cm.InstallTestCertificate(ctx, env)
		Expect(err).To(BeNil())

		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cert-manager.io",
			Kind:    "Certificate",
			Version: "v1",
		})

		Eventually(func() error {
			err := k8sClient.Get(ctx,
				ctrlClient.ObjectKey{
					Namespace: cm.Name(),
					Name:      "test-certificate",
				}, cert)
			if err != nil {
				return err
			}

			conditions, _, _ := unstructured.NestedSlice(cert.Object, "status", "conditions")
			for _, c := range conditions {
				condition := c.(map[string]interface{})
				if condition["type"] == "Ready" && condition["status"] == "True" {
					return nil
				}
			}
			return fmt.Errorf("certificate not ready yet")

		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})
})

var _ = Describe("Upgrading Cert Manager Test", Ordered, Label("cert-manager", "upgrade"), func() {
	var (
		cm certManager
		hr *fluxhelmv2beta2.HelmRelease
	)

	It("should install the previous version successfully", func() {
		cm = certManager{}
		err := cm.InstallPreviousVersion(ctx, env)
		Expect(err).To(BeNil())

		hr = &fluxhelmv2beta2.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       fluxhelmv2beta2.HelmReleaseKind,
				APIVersion: fluxhelmv2beta2.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.Name(),
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

	It("should create the root CA successfully", func() {
		err := cm.InstallPreviousVersionRootCA(ctx, env)
		Expect(err).To(BeNil())

		rootCA := &unstructured.Unstructured{}
		rootCA.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cert-manager.io",
			Kind:    "ClusterIssuer",
			Version: "v1",
		})

		Eventually(func() error {
			err := k8sClient.Get(ctx,
				ctrlClient.ObjectKey{
					Name: "kommander-ca",
				}, rootCA)
			if err != nil {
				return err
			}
			conditions, _, _ := unstructured.NestedSlice(rootCA.Object, "status", "conditions")

			for _, c := range conditions {
				condition := c.(map[string]interface{})
				if condition["type"] == "Ready" && condition["status"] == "True" {
					return nil
				}
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should upgrade cert-manager successfully", func() {
		err := cm.Upgrade(ctx, env)
		Expect(err).To(BeNil())

		hr = &fluxhelmv2beta2.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				Kind:       fluxhelmv2beta2.HelmReleaseKind,
				APIVersion: fluxhelmv2beta2.GroupVersion.Version,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.Name(),
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

	It("should should upgrade the Root CA successfully", func() {
		err := cm.UpgradeRootCA(ctx, env)
		Expect(err).To(BeNil())

		rootCA := &unstructured.Unstructured{}
		rootCA.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cert-manager.io",
			Kind:    "ClusterIssuer",
			Version: "v1",
		})

		Eventually(func() error {
			err := k8sClient.Get(ctx,
				ctrlClient.ObjectKey{
					Name: "kommander-ca",
				}, rootCA)
			if err != nil {
				return err
			}
			conditions, _, _ := unstructured.NestedSlice(rootCA.Object, "status", "conditions")

			for _, c := range conditions {
				condition := c.(map[string]interface{})
				if condition["type"] == "Ready" && condition["status"] == "True" {
					return nil
				}
			}
			return nil
		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})

	It("should issue a certificate successfully after upgrade", func() {
		err := cm.InstallTestCertificate(ctx, env)
		Expect(err).To(BeNil())

		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "cert-manager.io",
			Kind:    "Certificate",
			Version: "v1",
		})

		Eventually(func() error {
			err := k8sClient.Get(ctx,
				ctrlClient.ObjectKey{
					Namespace: cm.Name(),
					Name:      "test-certificate",
				}, cert)
			if err != nil {
				return err
			}

			conditions, _, _ := unstructured.NestedSlice(cert.Object, "status", "conditions")
			for _, c := range conditions {
				condition := c.(map[string]interface{})
				if condition["type"] == "Ready" && condition["status"] == "True" {
					return nil
				}
			}
			return fmt.Errorf("certificate not ready yet")

		}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
	})
})
