// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"fmt"
	"time"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// RegisterDefaultTests programmatically registers a Ginkgo Describe block
// for the named application with install and upgrade sub-tests. This is the
// "template test" used for any catalog app that has no dedicated _test.go.
//
// The generated tests:
//   - create a Kind cluster (or reuse E2E_KUBECONFIG)
//   - install the app via helmrelease kustomization
//   - auto-detect reconciliation target:
//   - HelmRelease/<releaseName> OR Flux Kustomization/<releaseName>-kustomize
//   - if a previous version exists: install it, upgrade, assert success
func RegisterDefaultTests(appName string) {
	var _ = Describe(appName+" Tests", Label(appName), func() {
		Describe("Installing "+appName, Ordered, Label("install"), func() {
			var app *App

			BeforeAll(func() {
				err := SetupKindCluster()
				Expect(err).ToNot(HaveOccurred())

				err = Env.InstallLatestFlux(Ctx)
				Expect(err).ToNot(HaveOccurred())

				err = ApplyFluxHelmControllerFeatureGates()
				Expect(err).ToNot(HaveOccurred())

				err = WaitForFluxCRDs()
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				Expect(TeardownCluster()).To(Succeed())
			})

			It("should install successfully with default config", func() {
				app = NewAppScenario(appName, *AppVersion).(*App)
				GinkgoWriter.Printf("Installing %s @ %s\n", app.Name(), *AppVersion)
				err := app.Install(Ctx, Env)
				Expect(err).ToNot(HaveOccurred())
				Expect(waitForDefaultReconcilerReady(app.Name(), 10*time.Minute)).To(Succeed())
			})
		})

		Describe("Upgrading "+appName, Ordered, Label("upgrade"), func() {
			var app *App

			BeforeAll(func() {
				app = NewAppScenario(appName, *AppVersion).(*App)
				if !app.HasPreviousVersion() {
					Skip("skipping upgrade test: no previous version available")
				}

				err := SetupKindCluster()
				Expect(err).ToNot(HaveOccurred())

				err = Env.InstallLatestFlux(Ctx)
				Expect(err).ToNot(HaveOccurred())

				err = ApplyFluxHelmControllerFeatureGates()
				Expect(err).ToNot(HaveOccurred())

				err = WaitForFluxCRDs()
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				Expect(TeardownCluster()).To(Succeed())
			})

			It("should install the previous version successfully", func() {
				err := app.InstallPreviousVersion(Ctx, Env)
				Expect(err).ToNot(HaveOccurred())
				Expect(waitForDefaultReconcilerReady(app.Name(), 10*time.Minute)).To(Succeed())
			})

			It("should upgrade "+appName+" successfully", func() {
				err := app.Upgrade(Ctx, Env)
				Expect(err).ToNot(HaveOccurred())
				Expect(waitForDefaultReconcilerReady(app.Name(), 10*time.Minute)).To(Succeed())
			})
		})
	})
}

func waitForDefaultReconcilerReady(appName string, timeout time.Duration) error {
	reconciler, err := detectDefaultReconciler(appName)
	if err != nil {
		return err
	}

	switch reconciler {
	case "helmrelease":
		return waitForHelmReleaseReady(appName, timeout)
	case "kustomization":
		// For Kustomize apps, source + kustomization readiness gives the same
		// signal as HelmRelease readiness in Helm-based apps.
		if err := waitForGitRepositoryReady(appName+"-manifests", timeout); err != nil {
			return err
		}
		return waitForFluxKustomizationReady(appName+"-kustomize", timeout)
	default:
		return fmt.Errorf("unknown reconciler type %q", reconciler)
	}
}

func detectDefaultReconciler(appName string) (string, error) {
	hr := &fluxhelmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: DefaultNamespace},
	}
	err := K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
	if err == nil {
		return "helmrelease", nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("checking helmrelease reconciler for %s: %w", appName, err)
	}

	ksName := appName + "-kustomize"
	ks := &unstructured.Unstructured{}
	ks.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization",
	})
	ks.SetName(ksName)
	ks.SetNamespace(DefaultNamespace)
	err = K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(ks), ks)
	if err == nil {
		return "kustomization", nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return "", fmt.Errorf("checking flux kustomization reconciler for %s: %w", appName, err)
	}

	return "", fmt.Errorf("unable to detect reconciler for app %q: expected HelmRelease/%s or Kustomization/%s in namespace %s", appName, appName, ksName, DefaultNamespace)
}

func waitForHelmReleaseReady(appName string, timeout time.Duration) error {
	hr := &fluxhelmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: DefaultNamespace},
	}

	var waitErr error
	Eventually(func() error {
		waitErr = K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
		if waitErr != nil {
			GinkgoWriter.Printf("HelmRelease Get error: %v\n", waitErr)
			return waitErr
		}

		GinkgoWriter.Printf("HelmRelease %s/%s conditions: %v\n", hr.Namespace, hr.Name, hr.Status.Conditions)
		for _, cond := range hr.Status.Conditions {
			if cond.Status == metav1.ConditionTrue && cond.Type == apimeta.ReadyCondition {
				GinkgoWriter.Printf("HelmRelease is Ready!\n")
				return nil
			}
		}
		return fmt.Errorf("helm release not ready yet")
	}).WithPolling(PollInterval).WithTimeout(timeout).Should(Succeed())

	return waitErr
}

func waitForGitRepositoryReady(name string, timeout time.Duration) error {
	gr := &unstructured.Unstructured{}
	gr.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository",
	})
	gr.SetName(name)
	gr.SetNamespace(DefaultNamespace)

	return waitForUnstructuredReady(gr, timeout)
}

func waitForFluxKustomizationReady(name string, timeout time.Duration) error {
	ks := &unstructured.Unstructured{}
	ks.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization",
	})
	ks.SetName(name)
	ks.SetNamespace(DefaultNamespace)

	return waitForUnstructuredReady(ks, timeout)
}

func waitForUnstructuredReady(obj *unstructured.Unstructured, timeout time.Duration) error {
	var waitErr error
	Eventually(func() error {
		waitErr = K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(obj), obj)
		if waitErr != nil {
			GinkgoWriter.Printf("%s Get error: %v\n", obj.GetKind(), waitErr)
			return waitErr
		}

		conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%s %s/%s has no status.conditions yet", obj.GetKind(), obj.GetNamespace(), obj.GetName())
		}

		GinkgoWriter.Printf("%s %s/%s conditions: %v\n", obj.GetKind(), obj.GetNamespace(), obj.GetName(), conditions)
		for _, c := range conditions {
			conditionMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if conditionMap["type"] == string(apimeta.ReadyCondition) && conditionMap["status"] == string(metav1.ConditionTrue) {
				GinkgoWriter.Printf("%s is Ready!\n", obj.GetKind())
				return nil
			}
		}
		return fmt.Errorf("%s not ready yet", obj.GetKind())
	}).WithPolling(PollInterval).WithTimeout(timeout).Should(Succeed())
	return waitErr
}
