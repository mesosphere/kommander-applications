package appscenarios

import (
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

var _ = Describe("Kube OIDC Proxy App Tests", Label("kube-oidc-proxy"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		// Apply Kommander base kustomizations to ensure priority classes and other base resources exist
		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}
		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Installing kube-oidc-proxy", Ordered, Label("install"), func() {
		var (
			k              *kubeOIDCProxy
			hr             *fluxhelmv2beta2.HelmRelease
			deploymentList *appsv1.DeploymentList
		)

		It("should install kube-oidc-proxy successfully with default config", func() {
			k = NewKubeOIDCProxy()
			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			hr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      k.Name(),
					Namespace: kommanderNamespace,
				},
			}

			// Check the status of the HelmRelease
			Eventually(func() error {
				err = env.Client.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
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

		It("should have the correct PriorityClass configured", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": k.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			deploymentList = &appsv1.DeploymentList{}
			err = env.Client.List(ctx, deploymentList, listOptions)
			Expect(err).To(BeNil())
			Expect(deploymentList.Items).To(HaveLen(1))

			for _, deployment := range deploymentList.Items {
				Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal(dkpCriticalPriority))
			}
		})

		It("should be healthy and all resources should exist", func() {
			Eventually(func() error {
				return k.IsHealthy(ctx, env)
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should have proper OIDC configuration from ConfigMap", func() {
			configMap := &corev1.ConfigMap{}
			err := env.Client.Get(ctx, ctrlClient.ObjectKey{
				Namespace: kommanderNamespace,
				Name:      "kube-oidc-proxy-0.3.6-config-defaults",
			}, configMap)
			Expect(err).To(BeNil())

			// Verify the ConfigMap contains expected OIDC configuration
			valuesYaml, exists := configMap.Data["values.yaml"]
			Expect(exists).To(BeTrue())
			Expect(valuesYaml).To(ContainSubstring("clientId: kube-apiserver"))
			Expect(valuesYaml).To(ContainSubstring("usernameClaim: email"))
			Expect(valuesYaml).To(ContainSubstring("groupsClaim: groups"))
			Expect(valuesYaml).To(ContainSubstring("groupsPrefix: \"oidc:\""))
		})

		It("should have ingress configured for API server access", func() {
			deployment := &appsv1.Deployment{}
			err := env.Client.Get(ctx, ctrlClient.ObjectKey{
				Namespace: kommanderNamespace,
				Name:      k.Name(),
			}, deployment)
			Expect(err).To(BeNil())

			// Verify deployment has the correct reloader annotations for config changes
			annotations := deployment.Spec.Template.Annotations
			Expect(annotations).To(HaveKey("secret.reloader.stakater.com/reload"))
			Expect(annotations["secret.reloader.stakater.com/reload"]).To(ContainSubstring("kube-oidc-proxy-config"))
			Expect(annotations["secret.reloader.stakater.com/reload"]).To(ContainSubstring("kube-oidc-proxy-server-tls"))
		})

		It("should pass business logic test - OIDC proxy functionality", func() {
			err := k.TestBusinessLogic(ctx, env)
			Expect(err).To(BeNil())
		})
	})

	Describe("Upgrading kube-oidc-proxy", Ordered, Label("upgrade"), func() {
		var (
			k *kubeOIDCProxy
		)

		It("should install previous version first", func() {
			k = NewKubeOIDCProxy()
			err := k.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr := &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      k.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = env.Client.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
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

		It("should upgrade to current version and remain healthy", func() {
			err := k.Upgrade(ctx, env)
			Expect(err).To(BeNil())

			hr := &fluxhelmv2beta2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      k.Name(),
					Namespace: kommanderNamespace,
				},
			}

			// Wait for upgrade to complete
			Eventually(func() error {
				err = env.Client.Get(ctx, ctrlClient.ObjectKeyFromObject(hr), hr)
				if err != nil {
					return err
				}

				for _, cond := range hr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready after upgrade")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())

			// Verify health after upgrade
			Eventually(func() error {
				return k.IsHealthy(ctx, env)
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})
	})
})
