package environment

import (
	"context"
	"embed"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/gomega" //nolint:stylecheck,revive // test code

	helmclient "github.com/mittwald/go-helm-client"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/mesosphere/kommander-applications/apptests/net"
	"github.com/mesosphere/kommander-applications/apptests/utils"
)

//go:embed metallb-crs/*.yaml
var metallbCRs embed.FS

// InstallMetallb runs helm installation of metallb chart with configuration to use
// IP addresses from given subnet. The function return expected traefik load balancer
// address.
func InstallMetallb(ctx context.Context, kubeconfigPath string, subnet *net.Subnet) netaddr.IP {
	ok, addresses := subnet.NextRange()
	Expect(ok).Should(BeTrue(), "not able to get next address range")

	addressRange, err := netaddr.ParseIPRange(addresses)
	Expect(err).ShouldNot(HaveOccurred())

	chartPath := "charts/metallb-0.13.7.tgz"

	kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	Expect(err).ShouldNot(HaveOccurred())

	opt := &helmclient.KubeConfClientOptions{
		Options: &helmclient.Options{
			Namespace:        "metallb-system", // Change this to the namespace you wish to install the chart in.
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            true,
			Linting:          false,
			DebugLog: func(format string, v ...interface{}) {
				fmt.Printf(format+"\n", v...)
			},
		},
		KubeContext: "",
		KubeConfig:  kubeconfigBytes,
	}
	helmClient, err := helmclient.NewClientFromKubeConf(opt)
	Expect(err).ShouldNot(HaveOccurred())

	timeout := 5 * time.Minute
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}
	chartSpec := helmclient.ChartSpec{
		ReleaseName:     "metallb",
		ChartName:       chartPath,
		Namespace:       "metallb-system",
		CreateNamespace: true,
		UpgradeCRDs:     true,
		Wait:            true,
		Timeout:         timeout,
	}

	_, err = helmClient.InstallOrUpgradeChart(ctx, &chartSpec, nil)
	Expect(err).ShouldNot(HaveOccurred())

	cl, err := NewClient(kubeconfigPath)
	Expect(err).ShouldNot(HaveOccurred())
	for _, file := range []string{
		"metallb-crs/ipaddresspool.yaml",
		"metallb-crs/l2advertisement.yaml",
	} {
		content, err := metallbCRs.ReadFile(file)
		Expect(err).ShouldNot(HaveOccurred())
		content, err = utils.EnvsubstBytes(content, utils.SubstitionsFromMap(map[string]string{
			"addresses": addresses,
		}))
		Expect(err).ShouldNot(HaveOccurred())
		u := &unstructured.Unstructured{Object: map[string]interface{}{}}
		Expect(yaml.Unmarshal(content, &u)).To(Succeed())
		err = cl.Create(ctx, u)
		Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())
	}

	return addressRange.From()
}

// NewClient returns a new Client using the provided kube config path.
func NewClient(kubeConfigPath string) (client.Client, error) {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	return k8sClient, nil
}
