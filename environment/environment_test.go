package environment

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/nutanix-cloud-native/nkp-catalog-tests/flux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProvision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping kind cluster provision in short mode")
	}

	env := Env{}
	ctx := context.Background()

	err := env.Provision(ctx)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, env.Destroy(ctx))
	}()

	selector := labels.SelectorFromSet(map[string]string{
		manifestgen.PartOfLabelKey:   manifestgen.PartOfLabelValue,
		manifestgen.InstanceLabelKey: kommanderFluxNamespace,
	})

	// get flux deployments
	deployments, err := env.K8sClient.Clientset().AppsV1().
		Deployments(kommanderFluxNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
	assert.NoError(t, err)

	// assert that there are 3 deployments(helm-controller, kustomize-controller, source-controller)
	assert.Equal(t, 3, len(deployments.Items))

	// assert that flux deployments are ready
	for _, deployment := range deployments.Items {
		deploymentObj, err := env.K8sClient.Clientset().AppsV1().Deployments(kommanderFluxNamespace).
			Get(ctx, deployment.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, deploymentObj.Status.Replicas, deploymentObj.Status.ReadyReplicas)
	}
}

func TestApplyKustomizations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping kind cluster kustomize apply in short mode")
	}

	ctx := context.Background()
	env := &Env{}

	kustomizePath, err := absolutePathToBase()
	require.NoError(t, err)
	fmt.Println(kustomizePath)

	cluster, k8sClient, err := provisionEnv(ctx, nil)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, env.Destroy(ctx))
	}()

	c, err := genericCLient.New(k8sClient.Config(), genericCLient.Options{
		Scheme: flux.NewScheme(),
	})
	assert.NoError(t, err)

	env.SetClient(c)
	env.SetCluster(cluster)

	// apply common/base kustomizations
	err = env.ApplyKustomizations(ctx, kustomizePath, nil)
	assert.NoError(t, err)

	// assert that following HelmRepository (as an example) is created
	hr := &sourcev1.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "source.toolkit.fluxcd.io",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vmware-tanzu.github.io",
			Namespace: kommanderFluxNamespace,
		},
	}

	client, err := genericCLient.New(env.K8sClient.Config(), genericCLient.Options{Scheme: flux.NewScheme()})
	assert.NoError(t, err)

	// set timeout on the context
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// assert that eventually helmRelease object is reconciled
	err = wait.PollUntilContextCancel(ctx, pollInterval, true, func(ctx context.Context) (done bool, err error) {
		err = client.Get(ctx, genericCLient.ObjectKeyFromObject(hr), hr)
		if err != nil {
			return false, err
		}
		for _, cond := range hr.Status.Conditions {
			if cond.Status == metav1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, hr)
}

func TestAbsolutePathToBase(t *testing.T) {
	t.Setenv("NKP_CATALOG_DIR", "/tmp/nkp-catalog")

	pathToBase, err := absolutePathToBase()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join("/tmp/nkp-catalog", "common", "base"), pathToBase)
}
