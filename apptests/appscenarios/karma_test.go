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
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	karmaTlsCertSecretName = "karma-client-tls-cert"
)

var (
	k                   *karma
	karmaHr             *fluxhelmv2beta2.HelmRelease
	karmaDeploymentList *appsv1.DeploymentList
	karmaContainer      corev1.Container
)

var _ = Describe("Karma Install Test", Ordered, Label("karma", "install"), func() {

	It("should install successfully with default config", func() {
		k = NewKarma()
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
			Expect(karmaDeploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal(dkpHighPriority))

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
				if vm.Name == "karma-config" {
					found = true
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Context("Karma Service", func() {

	})

	Context("Karma Ingress", func() {

	})

	Context("Karma ConfigMap", func() {

	})

})
