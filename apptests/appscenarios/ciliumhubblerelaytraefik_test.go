package appscenarios

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fluxhelmv2beta2 "github.com/fluxcd/helm-controller/api/v2beta2"
	apimeta "github.com/fluxcd/pkg/apis/meta"
	"github.com/mesosphere/kommander-applications/apptests/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cilium Hubble Relay Traefik Tests", Label(constants.CiliumHubbleRelayTraefik), func() {
	var c *ciliumHubbleRelayTraefik

	BeforeEach(OncePerOrdered, func() {
		err := SetupKindCluster()
		Expect(err).To(BeNil())

		err = env.InstallLatestFlux(ctx)
		Expect(err).To(BeNil())

		err = env.ApplyKommanderBaseKustomizations(ctx)
		Expect(err).To(BeNil())

		c = NewCiliumHubbleRelayTraefik()
	})

	AfterEach(OncePerOrdered, func() {
		if os.Getenv("SKIP_CLUSTER_TEARDOWN") != "" {
			return
		}

		err := env.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Cilium Hubble Relay Traefik Install Test", Ordered, Label("install"), func() {
		var ciliumHubbleRelayHR *fluxhelmv2beta2.HelmRelease

		It("should install Cilium Hubble Relay Traefik dependencies", func() {
			installCiliumHubbleRelayTraefikDependencies()
		})

		It("should install successfully with default config", func() {
			err := c.Install(ctx, env)
			Expect(err).To(BeNil())

			ciliumHubbleRelayHR = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      c.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(ciliumHubbleRelayHR), ciliumHubbleRelayHR)
				if err != nil {
					return err
				}

				for _, cond := range ciliumHubbleRelayHR.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should forward requests to Hubble Relay", func() {
			connectToHubbleRelayViaLoadbalancer()
		})
	})

	PDescribe("Cilium Hubble Relay Traefik Upgrade Test", Ordered, Label("upgrade"), func() {
		var ciliumHubbleRelayHR *fluxhelmv2beta2.HelmRelease

		It("should install Cilium Hubble Relay Traefik dependencies", func() {
			installCiliumHubbleRelayTraefikDependencies()
		})

		It("should install previous version successfully with default config", func() {
			err := c.InstallPreviousVersion(ctx, env)
			Expect(err).To(BeNil())

			ciliumHubbleRelayHR = &fluxhelmv2beta2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       fluxhelmv2beta2.HelmReleaseKind,
					APIVersion: fluxhelmv2beta2.GroupVersion.Version,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      c.Name(),
					Namespace: kommanderNamespace,
				},
			}

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(ciliumHubbleRelayHR), ciliumHubbleRelayHR)
				if err != nil {
					return err
				}

				for _, cond := range ciliumHubbleRelayHR.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should forward requests to Hubble Relay before upgrade", func() {
			connectToHubbleRelayViaLoadbalancer()
		})

		It("should upgrade successfully", func() {
			err := c.Install(ctx, env)
			Expect(err).To(BeNil())

			Eventually(func() error {
				err := k8sClient.Get(ctx, ctrlClient.ObjectKeyFromObject(ciliumHubbleRelayHR), ciliumHubbleRelayHR)
				if err != nil {
					return err
				}

				for _, cond := range ciliumHubbleRelayHR.Status.Conditions {
					if cond.Status == metav1.ConditionTrue &&
						cond.Type == apimeta.ReadyCondition {
						return nil
					}
				}
				return fmt.Errorf("helm release not ready yet")
			}).WithPolling(pollInterval).WithTimeout(5 * time.Minute).Should(Succeed())
		})

		It("should forward requests to Hubble Relay after upgrade", func() {
			connectToHubbleRelayViaLoadbalancer()
		})
	})
})

func installCiliumHubbleRelayTraefikDependencies() {
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

	By("Verifying that cert-manager CRDs are installed")
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

	By("Installing traefik")
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

	By("Installing a fake helm relay server")
	testDataPath, err := getTestDataDir()
	Expect(err).To(BeNil())

	fakeHubbleRelayManifest := filepath.Join(testDataPath, "cilium-hubble-relay-traefik", "hubble-relay-fake.yaml")
	err = env.ApplyYAML(ctx, fakeHubbleRelayManifest, map[string]string{})
	Expect(err).To(BeNil())
}

func connectToHubbleRelayViaLoadbalancer() {
	// Wait for the loadbalancer IP
	traefikSvc := &corev1.Service{}
	Eventually(func(g Gomega) {
		err := k8sClient.Get(
			ctx,
			types.NamespacedName{
				Name:      "kommander-traefik",
				Namespace: "kommander",
			}, traefikSvc,
		)
		g.Expect(err).To(BeNil())
		g.Expect(
			len(traefikSvc.Status.LoadBalancer.Ingress)).To(BeNumerically(">=", 1),
			"loadbalancer IP not available",
		)
	}).WithPolling(pollInterval).WithTimeout(2 * time.Minute).Should(Succeed())
	loadbalancerIP := traefikSvc.Status.LoadBalancer.Ingress[0].IP

	// Make the request
	hubbleRelayURL := url.URL{
		Scheme: "https",
		Host:   "hubble.hubble-relay.cilium.io:443",
	}

	Eventually(func(g Gomega) {
		// Make the request
		req, err := http.NewRequest("GET", hubbleRelayURL.String(), nil)
		g.Expect(err).To(BeNil())

		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if addr == hubbleRelayURL.Host {
						// The request host is used for the SNI header, but the actual address is the loadbalancer
						addr = fmt.Sprintf("%s:443", loadbalancerIP)
					}
					return dialer.DialContext(ctx, network, addr)
				},
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // The server certificate is self-signed
				},
			},
		}
		resp, err := client.Do(req)
		g.Expect(err).To(BeNil())
		defer resp.Body.Close()

		// Validate the response
		g.Expect(resp.StatusCode).To(Equal(200))
		g.Expect(len(resp.TLS.PeerCertificates)).To(BeNumerically(">", 0))
		g.Expect(resp.TLS.PeerCertificates[0].Issuer.CommonName).To(Equal("Hubble Relay Fake - ECC Intermediate"))
	}).WithPolling(pollInterval).WithTimeout(1 * time.Minute).Should(Succeed())
}
