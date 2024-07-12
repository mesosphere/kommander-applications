package appscenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	"github.com/mesosphere/kommander-applications/apptests/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	karmaTlsCertSecretName = "kommander-karma-client-tls"
	karmaConfigMapName     = "karma-config"
	traefikOverrideCMName  = "traefik-overrides"
)

var _ = Describe("Karma Tests", Label("karma"), func() {
	var (
		k *karma
	)

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		k = NewKarma()
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Karma Install Test", Ordered, Label("install"), func() {
		var (
			karmaHr             *fluxhelmv2beta2.HelmRelease
			karmaDeploymentList *appsv1.DeploymentList
			karmaContainer      corev1.Container
		)

		It("should install karma dependencies", func() {
			installKarmaDependencies(k)
		})

		It("should install successfully with default config", func() {

			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			karmaHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      k.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(karmaHr), karmaHr)
				if err != nil {
					return err
				}

				for _, cond := range karmaHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		Context("Karma Deployment", func() {
			It("should have empty resource limits and priority class", func() {
				selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
					MatchLabels: map[string]string{
						"helm.toolkit.fluxcd.io/name": k.Name(),
					},
				})
				Expect(err).To(BeNil())
				listOptions := &ctrlClient.ListOptions{
					LabelSelector: selector,
				}
				karmaDeploymentList = &appsv1.DeploymentList{}
				err = k8sClient.List(ctx, karmaDeploymentList, listOptions)
				Expect(err).To(BeNil())
				Expect(karmaDeploymentList.Items).To(HaveLen(1))
				Expect(karmaDeploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal(dkpCriticalPriority))

				karmaContainer = karmaDeploymentList.Items[0].Spec.Template.Spec.Containers[0]
				Expect(karmaContainer.Resources.Requests).To(BeEmpty())
				Expect(karmaContainer.Resources.Limits).To(BeEmpty())
			})

			It("should override the readiness probe", func() {
				Expect(karmaContainer.ReadinessProbe).ToNot(BeNil())
				Expect(karmaContainer.ReadinessProbe.HTTPGet).ToNot(BeNil())
				Expect(karmaContainer.ReadinessProbe.HTTPGet.Path).To(Equal("/dkp/kommander/monitoring/karma/"))
			})

			It("should mount secret based client tls cert", func() {
				found := false
				for _, vm := range karmaContainer.VolumeMounts {
					if vm.Name == karmaTlsCertSecretName {
						found = true
					}
				}
				Expect(found).To(BeTrue())
			})

			It("should mount configmap based configuration", func() {
				found := false
				for _, vm := range karmaContainer.VolumeMounts {
					if vm.Name == karmaConfigMapName {
						found = true
					}
				}
				Expect(found).To(BeTrue())
			})

			It("should have reloader annotations about cm and secret", func() {
				karmaDeployment := karmaDeploymentList.Items[0]
				Expect(karmaDeployment.Annotations).To(HaveKeyWithValue("configmap.reloader.stakater.com/reload", karmaConfigMapName))
				Expect(karmaDeployment.Annotations).To(HaveKeyWithValue("secret.reloader.stakater.com/reload", karmaTlsCertSecretName))
			})

		})

		Context("Karma Service", func() {
			It("should have prometheus label set", func() {
				karmaSvc := &corev1.Service{}
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(karmaHr), karmaSvc)
				Expect(err).To(BeNil())
				Expect(karmaSvc.Labels).To(HaveKeyWithValue("servicemonitor.kommander.mesosphere.io/path", "dkp__kommander__monitoring__karma__metrics"))
			})
		})

		Context("Karma Ingress", func() {
			karmaIngress := &networking.Ingress{}
			It("should have traefik ingress annotations", func() {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(karmaHr), karmaIngress)
				karmaTfkMdlwaConfigStr := fmt.Sprintf("%s-stripprefixes@kubernetescrd,%s-forwardauth@kubernetescrd", kommanderNamespace, kommanderNamespace)
				Expect(err).To(BeNil())
				Expect(karmaIngress.Annotations).To(HaveKeyWithValue("kubernetes.io/ingress.class", "kommander-traefik"))
				Expect(karmaIngress.Annotations).To(HaveKeyWithValue("traefik.ingress.kubernetes.io/router.tls", "true"))
				Expect(karmaIngress.Annotations).To(HaveKeyWithValue("traefik.ingress.kubernetes.io/router.middlewares",
					karmaTfkMdlwaConfigStr))
			})

			It("should set the correct path", func() {
				Expect(karmaIngress.Spec.Rules[0].HTTP.Paths[0].Path).To(Equal("/dkp/kommander/monitoring/karma"))
			})
		})

		Context("Karma ConfigMap", func() {
			karmaConfigMap := &corev1.ConfigMap{}
			It("should have the helm annotations", func() {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKey{Namespace: kommanderNamespace, Name: karmaConfigMapName}, karmaConfigMap)
				Expect(err).To(BeNil())
				Expect(karmaConfigMap.Annotations).To(HaveKeyWithValue("helm.sh/hook", "pre-install"))
				Expect(karmaConfigMap.Annotations).To(HaveKeyWithValue("helm.sh/hook-delete-policy", "before-hook-creation"))
			})
		})

		Context("Karma Availability", func() {
			It("should have access to the karma dashboard", func() {
				selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": "karma",
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

				res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":8080").SubResource("proxy").Suffix("/dkp/kommander/monitoring/karma/").Do(ctx)
				Expect(res.Error()).To(BeNil())

				var statusCode int
				res.StatusCode(&statusCode)
				Expect(statusCode).To(Equal(200))
			})
		})
	})

	Describe("Karma Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			karmaHr *fluxhelmv2beta2.HelmRelease
		)

		It("should install karma dependencies successfully", func() {
			installKarmaDependencies(k)
		})

		It("should install previous version successfully with default config", func() {
			err := k.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			karmaHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      k.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(karmaHr), karmaHr)
				if err != nil {
					return err
				}

				for _, cond := range karmaHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should have access to the karma dashboard", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "karma",
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

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":8080").SubResource("proxy").Suffix("/dkp/kommander/monitoring/karma/").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))
		})

		It("should upgrade karma successfully", func() {
			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(karmaHr), karmaHr)
				if err != nil {
					return err
				}

				for _, cond := range karmaHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should have access to the karma dashboard after upgrade", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "karma",
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

			res := restClientV1Pods.Get().Resource("pods").Namespace(podList.Items[0].Namespace).Name(podList.Items[0].Name + ":8080").SubResource("proxy").Suffix("/dkp/kommander/monitoring/karma/").Do(ctx)
			Expect(res.Error()).To(BeNil())

			var statusCode int
			res.StatusCode(&statusCode)
			Expect(statusCode).To(Equal(200))
		})
	})
})

func installKarmaDependencies(k *karma) {
	By("Installing cert-manager")
	cm := certManager{}
	err := cm.Install(ctx, env)
	Expect(err).To(BeNil())

	hr := &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.CertManager,
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

	By("Installing cert-manager crds successfully")
	certManagerCrds := &fluxhelmv2beta2.HelmRelease{
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
		err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(certManagerCrds), certManagerCrds)
		if err != nil {
			return err
		}

		for _, cond := range certManagerCrds.Status.Conditions {
			if cond.Status == metav1.ConditionTrue &&
				cond.Type == apimeta.ReadyCondition {
				return nil
			}
		}
		return fmt.Errorf("helm release not ready yet")
	}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

	By("Installing kommander-ca")
	testDataDir, err := getTestDataDir()
	Expect(err).To(BeNil())
	err = env.ApplyYAML(ctx, filepath.Join(testDataDir, "cert-manager/kommander-ca"), nil)
	Expect(err).To(BeNil())

	By("should install traefik")
	tfk := NewTraefik()
	err = tfk.Install(ctx, env)
	Expect(err).To(BeNil())

	hr = &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tfk.Name(),
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

	By("should install karma-traefik")
	err = k.InstallDependency(ctx, env, constants.KarmaTraefik)
	Expect(err).To(BeNil())

	hr = &fluxhelmv2beta2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxhelmv2beta2.HelmReleaseKind,
			APIVersion: fluxhelmv2beta2.GroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.KarmaTraefik,
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
}
