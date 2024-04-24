// Package kind provides function for creating and managing kind clusters.
package kind

import (
	"context"
	"os"
	"path/filepath"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

type Cluster struct {
	provider           *cluster.Provider
	kubeconfigFilePath string
	name               string
}

const (
	defaultClusterName = "kommanderapptest"
)

// CreateCluster creates a new kind cluster with the given name.
func CreateCluster(ctx context.Context, name string) (*Cluster, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	kubeconfigFile, err := os.CreateTemp("", "*-kubeconfig")
	if err != nil {
		return nil, err
	}

	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))
	if name == "" {
		name = defaultClusterName
	}

	kindConfigFile, err := os.ReadFile(filepath.Join("..", "kind/config/kind.yaml"))
	if err != nil {
		return nil, err
	}

	err = provider.Create(name,
		cluster.CreateWithKubeconfigPath(kubeconfigFile.Name()),
		cluster.CreateWithRawConfig(kindConfigFile),
	)

	if err != nil {
		return nil, err
	}

	// ExportKubeConfig exports the kubeconfig for the cluster with the given name to the standard output or a file.
	// This makes it easy for other applications, such as fluxcd, to work with the cluster.
	err = provider.ExportKubeConfig(name, "", false)
	if err != nil {
		return nil, err
	}

	return &Cluster{
		provider:           provider,
		kubeconfigFilePath: kubeconfigFile.Name(),
		name:               name,
	}, nil
}

// Delete deletes the cluster and the temporary kubeconfig file.
func (c *Cluster) Delete(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	err := c.provider.Delete(c.name, c.kubeconfigFilePath)
	if err != nil {
		return err
	}

	return os.Remove(c.kubeconfigFilePath)
}

func (c *Cluster) KubeconfigFilePath() string {
	return c.kubeconfigFilePath
}

func (c *Cluster) Name() string {
	return c.name
}
