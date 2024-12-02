package appscenarios

import (
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
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

var _ = Describe("Dex Tests", Label("dex"), func() {
	var (
		d *dex
	)

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		d = NewDex()
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Dex Install Test", Ordered, Label("install"), func() {
		var (
			deploymentList *appsv1.DeploymentList
			dexContainer   corev1.Container
		)

		It("should install dex dependencies", func() {
			installDexDependencies(d)
		})

		It("should install successfully with default config", func() {

			err := d.Install(ctx, env)
			fmt.Println("******* ctx and env *********", ctx, env)
			Expect(err).To(BeNil())
			dexHr := &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      d.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(dexHr), dexHr)
				if err != nil {
					return err
				}

				for _, cond := range dexHr.Status.Conditions {
					fmt.Printf("Condition Type: %s, Status: %s\n", cond.Type, cond.Status)
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(10 * time.Minute).Should(Succeed())
		})
		Context("Dex Ingress", func() {
			var (
				dexHr *fluxhelmv2beta2.HelmRelease
			)
			dexIngress := &networking.Ingress{}
			It("should have traefik ingress annotations", func() {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(dexHr), dexHr)
				//karmaTfkMdlwaConfigStr := fmt.Sprintf("%s-stripprefixes@kubernetescrd,%s-forwardauth@kubernetescrd", kommanderNamespace, kommanderNamespace)
				Expect(err).To(BeNil())
				Expect(dexIngress.Annotations).To(HaveKeyWithValue("kubernetes.io/ingress.class", "kommander-traefik"))
				Expect(dexIngress.Annotations).To(HaveKeyWithValue("traefik.ingress.kubernetes.io/router.tls", "true"))
				// Expect(karmaIngress.Annotations).To(HaveKeyWithValue("traefik.ingress.kubernetes.io/router.middlewares",
				// 	karmaTfkMdlwaConfigStr))
			})

			It("should set the correct path", func() {
				Expect(dexIngress.Spec.Rules[0].HTTP.Paths[0].Path).To(Equal("/dex"))
			})
		})

		It("should have resource limits and priority class", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": d.Name(),
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
			Expect(deploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal(dkpHighPriority))

			dexContainer = deploymentList.Items[0].Spec.Template.Spec.Containers[0]
			Expect(dexContainer.Resources.Requests.Cpu().String()).To(Equal("100m"))
			Expect(dexContainer.Resources.Requests.Memory().String()).To(Equal("128Mi"))
			Expect(dexContainer.Resources.Limits.Cpu().String()).To(Equal("100m"))
			Expect(dexContainer.Resources.Limits.Memory().String()).To(Equal("512Mi"))
		})

	})

	Describe("DEX Server Availability", func() {

		const issuerURL = "https://dex.kommander.svc.cluster.local:8080/dex"

		Context("testing the issuer URL", func() {
			It("should return a valid response from the DEX server", func() {
				// Make a request to the issuer URL
				resp, err := http.Get("https://dex.kommander.svc.cluster.local:8080/dex")

				// Ensure that we got a response without error
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))

			})
		})
	})

	Describe("Dex Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			dexHr *fluxhelmv2beta2.HelmRelease
		)

		It("should install dex dependencies successfully", func() {
			installDexDependencies(d)
		})

		It("should install previous version successfully with default config", func() {
			err := d.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			dexHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      d.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(dexHr), dexHr)
				if err != nil {
					return err
				}

				for _, cond := range dexHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should upgrade dex successfully", func() {
			err := d.Install(ctx, env)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(dexHr), dexHr)
				if err != nil {
					return err
				}

				for _, cond := range dexHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})
	})
})

func installDexDependencies(d *dex) {
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
		return fmt.Errorf("cert-manager helm release not ready yet")
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
		return fmt.Errorf(" cert-manager crds helm release not ready yet")
	}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

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
		return fmt.Errorf("traefik helm release not ready yet")
	}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

}
