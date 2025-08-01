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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("kube-oidc-proxy Tests", Ordered, Label("kube-oidc-proxy"), func() {
	var (
		ctx   = context.Background()
		proxy kubeOidcProxy
	)

	BeforeEach(OncePerOrdered, func() {
		Expect(SetupKindCluster()).To(Succeed())
		Expect(env.InstallLatestFlux(ctx)).To(Succeed())
		Expect(env.ApplyKommanderBaseKustomizations(ctx)).To(Succeed())
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") == "" {
			Expect(env.Destroy(ctx)).To(Succeed())
		}
	})

	Describe("Install", Ordered, Label("install"), func() {
		var (
			hr     *fluxhelmv2beta2.HelmRelease
			deploy *appsv1.Deployment
			ns     *corev1.Namespace
		)

		It("should create the kube-oidc-proxy namespace", func() {
			proxy = kubeOidcProxy{}
			Expect(proxy.Install(ctx, env)).To(Succeed())

			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: proxy.Name(),
				},
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(ns), ns)
			}, 3*time.Minute, pollInterval).Should(Succeed())
		})

		It("should install kube-oidc-proxy Helm release successfully", func() {
			hr = &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      proxy.Name(),
					Namespace: kommanderNamespace,
				},
			}
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				if err != nil {
					return err
				}
				for _, cond := range hr.Status.Conditions {
					if cond.Type == apimeta.ReadyCondition && cond.Status == metav1.ConditionTrue {
						return nil
					}
				}
				return fmt.Errorf("HelmRelease not ready yet")
			}, 5*time.Minute, pollInterval).Should(Succeed())
		})

		It("should deploy the kube-oidc-proxy deployment and have ready replicas", func() {
			deploy = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      proxy.Name(),
					Namespace: proxy.Name(),
				},
			}
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(deploy), deploy)
				if err != nil {
					return err
				}
				if deploy.Status.ReadyReplicas >= 1 {
					return nil
				}
				return fmt.Errorf("Deployment not ready yet")
			}, 3*time.Minute, pollInterval).Should(Succeed())
		})

		It("should mount the expected OIDC config secret", func() {
			secret := &corev1.Secret{}
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{
					Namespace: proxy.Name(),
					Name:      "kube-oidc-proxy-config",
				}, secret)
				if err != nil {
					return err
				}
				if secret.Data["oidc-issuer-url"] != nil {
					return nil
				}
				return fmt.Errorf("OIDC config secret missing oidc-issuer-url")
			}, 3*time.Minute, pollInterval).Should(Succeed())
		})
	})

	Describe("Upgrade", Ordered, Label("upgrade"), func() {
		var hr *fluxhelmv2beta2.HelmRelease

		It("should install the previous version", func() {
			proxy = kubeOidcProxy{}
			Expect(proxy.InstallPreviousVersion(ctx, env)).To(Succeed())

			hr = &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      proxy.Name(),
					Namespace: kommanderNamespace,
				},
			}
			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				if err != nil {
					return err
				}
				for _, cond := range hr.Status.Conditions {
					if cond.Type == apimeta.ReadyCondition && cond.Status == metav1.ConditionTrue {
						return nil
					}
				}
				return fmt.Errorf("Previous version HelmRelease not ready")
			}, 5*time.Minute, pollInterval).Should(Succeed())
		})

		It("should upgrade to the latest version", func() {
			Expect(proxy.Upgrade(ctx, env)).To(Succeed())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				if err != nil {
					return err
				}
				for _, cond := range hr.Status.Conditions {
					if cond.Type == apimeta.ReadyCondition && cond.Status == metav1.ConditionTrue {
						return nil
					}
				}
				return fmt.Errorf("HelmRelease not ready after upgrade")
			}, 5*time.Minute, pollInterval).Should(Succeed())
		})
	})
})
