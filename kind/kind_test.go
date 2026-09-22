package kind

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping kind cluster creation in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	name := "test-cluster"
	cluster, err := CreateCluster(ctx, name)
	require.NoError(t, err)
	require.NotNil(t, cluster)

	require.Equal(t, name, cluster.Name())
	require.NotEmpty(t, cluster.KubeconfigFilePath())

	require.NoError(t, cluster.Delete(ctx))
}
