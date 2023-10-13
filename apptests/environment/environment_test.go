package environment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	genericCLient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProvision(t *testing.T) {
	env := Env{}
	ctx := context.Background()

	err := env.Provision(ctx)
	assert.NoError(t, err)
	defer env.Destroy(ctx)

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
	ctx := context.Background()
	env := &Env{}

	// set the kustomizePath to common/base directory
	kustomizePath, err := absolutePathToBase()
	assert.NoError(t, err)
	fmt.Println(kustomizePath)

	// create a kind cluster and install fluxcd on it
	cluster, k8sClient, err := provisionEnv(ctx)
	assert.NoError(t, err)
	defer env.Destroy(ctx)

	env.SetK8sClient(k8sClient)
	env.SetCluster(cluster)

	// apply common/base kustomizations
	err = env.ApplyKustomizations(ctx, kustomizePath, nil)
	assert.NoError(t, err)

	// assert that following HelmRepository (as an example) is created
	hr := &sourcev1beta2.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "source.toolkit.fluxcd.io",
			APIVersion: "v1beta2",
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
	pathToBase, err := absolutePathToBase()
	assert.NoError(t, err)
	assert.Contains(t, pathToBase, "kommander-applications/common/base")
}
