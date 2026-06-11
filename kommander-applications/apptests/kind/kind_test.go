package kind

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// create a test cluster name
	name := "test-cluster"
	cluster, err := CreateCluster(ctx, name)

	// assert that there is no error and the cluster is not nil
	assert.NoError(t, err)
	assert.NotNil(t, cluster)

	// assert that the cluster has the expected name and kubeconfig file path
	assert.Equal(t, name, cluster.Name())
	assert.NotEmpty(t, cluster.KubeconfigFilePath())

	// delete the cluster
	err = cluster.Delete(ctx)
	assert.NoError(t, err)
}
