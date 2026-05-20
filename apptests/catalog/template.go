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
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// RegisterDefaultTests programmatically registers a Ginkgo Describe block
// for the named application with install and upgrade sub-tests. This is the
// "template test" used for any catalog app that has no dedicated _test.go.
//
// The generated tests:
//   - create a Kind cluster (or reuse E2E_KUBECONFIG)
//   - install the app via helmrelease kustomization
//   - poll HelmRelease until Ready
//   - if a previous version exists: install it, upgrade, assert success
func RegisterDefaultTests(appName string) {
	var _ = Describe(appName+" Tests", Label(appName), func() {
		Describe("Installing "+appName, Ordered, Label("install"), func() {
			var (
				app *App
				hr  *fluxhelmv2.HelmRelease
			)

			BeforeAll(func() {
				err := SetupKindCluster()
				Expect(err).ToNot(HaveOccurred())

				err = Env.InstallLatestFlux(Ctx)
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
				GinkgoWriter.Printf("Install applied, waiting for HelmRelease to become Ready\n")

				hr = &fluxhelmv2.HelmRelease{
					TypeMeta: metav1.TypeMeta{
						Kind:       fluxhelmv2.HelmReleaseKind,
						APIVersion: fluxhelmv2.GroupVersion.Version,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      app.Name(),
						Namespace: DefaultNamespace,
					},
				}

				Eventually(func() error {
					err = K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
					if err != nil {
						GinkgoWriter.Printf("HelmRelease Get error: %v\n", err)
						return err
					}

					GinkgoWriter.Printf("HelmRelease %s/%s conditions: %v\n",
						hr.Namespace, hr.Name, hr.Status.Conditions)

					for _, cond := range hr.Status.Conditions {
						if cond.Status == metav1.ConditionTrue &&
							cond.Type == apimeta.ReadyCondition {
							GinkgoWriter.Printf("HelmRelease is Ready!\n")
							return nil
						}
					}
					return fmt.Errorf("helm release not ready yet")
				}).WithPolling(PollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
			})
		})

		Describe("Upgrading "+appName, Ordered, Label("upgrade"), func() {
			var (
				app *App
				hr  *fluxhelmv2.HelmRelease
			)

			BeforeAll(func() {
				app = NewAppScenario(appName, *AppVersion).(*App)
				if !app.HasPreviousVersion() {
					Skip("skipping upgrade test: no previous version available")
				}

				err := SetupKindCluster()
				Expect(err).ToNot(HaveOccurred())

				err = Env.InstallLatestFlux(Ctx)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				Expect(TeardownCluster()).To(Succeed())
			})

			It("should install the previous version successfully", func() {
				err := app.InstallPreviousVersion(Ctx, Env)
				Expect(err).ToNot(HaveOccurred())

				hr = &fluxhelmv2.HelmRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      app.Name(),
						Namespace: DefaultNamespace,
					},
				}

				Eventually(func() error {
					err = K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
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
				}).WithPolling(PollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
			})

			It("should upgrade "+appName+" successfully", func() {
				err := app.Upgrade(Ctx, Env)
				Expect(err).ToNot(HaveOccurred())

				hr = &fluxhelmv2.HelmRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      app.Name(),
						Namespace: DefaultNamespace,
					},
				}

				Eventually(func() error {
					err = K8sClient.Get(Ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
					if err != nil {
						return err
					}
					for _, cond := range hr.Status.Conditions {
						if cond.Status == metav1.ConditionTrue &&
							cond.Type == apimeta.ReadyCondition &&
							cond.Reason == fluxhelmv2.UpgradeSucceededReason {
							return nil
						}
					}
					return fmt.Errorf("helm release not ready yet")
				}).WithPolling(PollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
			})
		})
	})
}
