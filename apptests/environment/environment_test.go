package environment

import (
	"context"
	"testing"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

func TestAbsolutePathToBase(t *testing.T) {
	s, err := absolutePathToBase()
	assert.NoError(t, err)
	assert.Contains(t, s, "kommander-applications/common/base")
}
