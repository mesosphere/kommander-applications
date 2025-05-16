package appscenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	"github.com/mesosphere/kommander-applications/apptests/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Kubernetes Dashboard Tests", Label("kubernetes-dashboard"), func() {
	var (
		k *kubernetesDashboard
	)
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		k = NewKubernetesDashboard()
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})
	Describe("Kubernetes Dashboard Install Test", Ordered, Label("install"), func() {
		var (
			kubernetesDashboardHR             *fluxhelmv2beta2.HelmRelease
			kubernetesDashboardDeploymentList *appsv1.DeploymentList
			kubernetesDashboardContainer      corev1.Container
		)

		It("should install kubernetes dashboard dependencies", func() {
			installKubernetesDashboardDependencies(k)
		})

		It("should install successfully with default config", func() {

			err := k.Install(ctx, env)
			Expect(err).To(BeNil())

			kubernetesDashboardHR = &fluxhelmv2beta2.HelmRelease{
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
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(kubernetesDashboardHR), kubernetesDashboardHR)
				if err != nil {
					return err
				}

				for _, cond := range kubernetesDashboardHR.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})
		// Assert the existence of resource limits and priority class
		It("should have resource limits and priority class", func() {
			selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"helm.toolkit.fluxcd.io/name": k.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			kubernetesDashboardDeploymentList = &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, kubernetesDashboardDeploymentList, listOptions)
			Expect(err).To(BeNil())
			Expect(kubernetesDashboardDeploymentList.Items).To(HaveLen(5))
			Expect(kubernetesDashboardDeploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal(dkpHighPriority))

			kubernetesDashboardContainer = kubernetesDashboardDeploymentList.Items[0].Spec.Template.Spec.Containers[0]
			Expect(kubernetesDashboardContainer.Resources.Requests.Cpu().String()).To(Equal("100m"))
			Expect(kubernetesDashboardContainer.Resources.Requests.Memory().String()).To(Equal("200Mi"))
			Expect(kubernetesDashboardContainer.Resources.Limits.Cpu().String()).To(Equal("500m"))
			Expect(kubernetesDashboardContainer.Resources.Limits.Memory().String()).To(Equal("1000Mi"))
		})
	})
})

func installKubernetesDashboardDependencies(k *kubernetesDashboard) {
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

}
