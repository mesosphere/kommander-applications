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
	"sigs.k8s.io/yaml"
)

var _ = Describe("Reloader Tests", Label("reloader"), func() {
	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())
	})

	AfterEach(OncePerOrdered, func() {
		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Reloader Install Test", Ordered, Label("install"), func() {
		var (
			r                      *reloader
			reloaderHr             *fluxhelmv2beta2.HelmRelease
			reloaderDeploymentList *appsv1.DeploymentList
			reloaderContainer      corev1.Container
		)

		It("should install successfully with default config", func() {
			r = NewReloader()
			err := r.Install(ctx, env)
			Expect(err).To(BeNil())

			reloaderHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      r.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(reloaderHr), reloaderHr)
				if err != nil {
					return err
				}

				for _, cond := range reloaderHr.Status.Conditions {
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
					"helm.toolkit.fluxcd.io/name": r.Name(),
				},
			})
			Expect(err).To(BeNil())
			listOptions := &ctrlClient.ListOptions{
				LabelSelector: selector,
			}
			reloaderDeploymentList = &appsv1.DeploymentList{}
			err = k8sClient.List(ctx, reloaderDeploymentList, listOptions)
			Expect(err).To(BeNil())
			Expect(reloaderDeploymentList.Items).To(HaveLen(1))
			Expect(reloaderDeploymentList.Items[0].Spec.Template.Spec.PriorityClassName).To(Equal(dkpHighPriority))

			reloaderContainer = reloaderDeploymentList.Items[0].Spec.Template.Spec.Containers[0]
			Expect(reloaderContainer.Resources.Requests.Cpu().String()).To(Equal("100m"))
			Expect(reloaderContainer.Resources.Requests.Memory().String()).To(Equal("128Mi"))
			Expect(reloaderContainer.Resources.Limits.Cpu().String()).To(Equal("100m"))
			Expect(reloaderContainer.Resources.Limits.Memory().String()).To(Equal("512Mi"))
		})

		It("should reload the application", func() {
			reloaderTestReload(r)
		})

	})

	Describe("Reloader Upgrade Test", Ordered, Label("upgrade"), func() {
		var (
			r          *reloader
			reloaderHr *fluxhelmv2beta2.HelmRelease
		)

		It("should install the previous version successfully", func() {
			r = NewReloader()
			err := r.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			hr := &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      r.Name(),
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

		})

		It("should upgrade reloader successfully", func() {
			// this is installing the latest version of the reloader
			err := r.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			reloaderHr = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      r.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(reloaderHr), reloaderHr)
				if err != nil {
					return err
				}

				for _, cond := range reloaderHr.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should reload the application", func() {
			reloaderTestReload(r)
		})
	})
})

func reloaderTestReload(r *reloader) {
	err := r.ApplyNginxConfigmap(ctx, env, "nginx-cm-old.yaml")
	Expect(err).To(BeNil())

	// deploy the nginx deployment
	deploymentYAML, err := os.ReadFile("../testdata/reloader/nginx.yaml")
	nginxDeployment := &appsv1.Deployment{}
	err = yaml.Unmarshal(deploymentYAML, nginxDeployment)
	nginxDeployment.SetNamespace(kommanderNamespace)
	nginxDeployment.SetAnnotations(map[string]string{
		"configmap.reloader.stakater.com/reload": nginxCMName,
	})
	err = k8sClient.Create(ctx, nginxDeployment)
	Expect(err).To(BeNil())

	Eventually(func() error {
		err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(nginxDeployment), nginxDeployment)
		if err != nil {
			return err
		}

		for _, cond := range nginxDeployment.Status.Conditions {
			if cond.Status == corev1.ConditionTrue &&
				cond.Type == appsv1.DeploymentAvailable {
				return nil
			}
		}
		return fmt.Errorf("deployment not ready yet")
	}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())

	// update the CM to break the deployment
	err = r.ApplyNginxConfigmap(ctx, env, "nginx-cm-new.yaml")
	Expect(err).To(BeNil())
	time.Sleep(1 * time.Second)

	// check if the deployment is updated and in a broken state
	Consistently(func() error {
		err = k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(nginxDeployment), nginxDeployment)
		if err != nil {
			return err
		}
		// after the nginx config update, the probe will fail thus breaks the deployment
		if nginxDeployment.Status.UpdatedReplicas == 1 &&
			nginxDeployment.Status.UnavailableReplicas == 1 {
			return nil
		}
		return fmt.Errorf("expected the deployment in a broken state")
	}, "5s").WithPolling(pollInterval).Should(Succeed())
}
