package environment

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/onsi/gomega"

	helmclient "github.com/mittwald/go-helm-client"
	"inet.af/netaddr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/mesosphere/kommander-applications/net"
	"github.com/mesosphere/kommander-applications/utils"
)

//go:embed metallb-crs/*.yaml
var metallbCRs embed.FS

//go:embed charts/*
var environmentChartsFS embed.FS

const METALLB_CHART_BUNDLE_NAME string = "metallb-0.13.7.tgz"

// InstallMetallb runs helm installation of metallb chart with configuration to use
// IP addresses from given subnet. The function return expected traefik load balancer
// address.
func InstallMetallb(ctx context.Context, kubeconfigPath string, subnet *net.Subnet) netaddr.IP {
	ok, addresses := subnet.NextRange()
	gomega.Expect(ok).Should(gomega.BeTrue(), "not able to get next address range")

	addressRange, err := netaddr.ParseIPRange(addresses)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	chartPath, err := GetEnvChartPath(METALLB_CHART_BUNDLE_NAME)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

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
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

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
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	cl, err := NewClient(kubeconfigPath)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	for _, file := range []string{
		"metallb-crs/ipaddresspool.yaml",
		"metallb-crs/l2advertisement.yaml",
	} {
		content, err := metallbCRs.ReadFile(file)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		content, err = utils.EnvsubstBytes(content, utils.SubstitionsFromMap(map[string]string{
			"addresses": addresses,
		}))
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		u := &unstructured.Unstructured{Object: map[string]interface{}{}}
		gomega.Expect(yaml.Unmarshal(content, &u)).To(gomega.Succeed())
		err = cl.Create(ctx, u)
		gomega.Expect(client.IgnoreAlreadyExists(err)).NotTo(gomega.HaveOccurred())
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

func GetEnvChartPath(fileName string) (string, error) {
	filePath := "charts/" + fileName
	content, err := environmentChartsFS.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Write to a temporary file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, filePath)

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(tmpFile), 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return "", err
	}

	return tmpFile, nil
}
