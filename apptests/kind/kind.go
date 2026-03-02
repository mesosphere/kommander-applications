// Package kind provides function for creating and managing kind clusters.
package kind

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

//go:embed config/kind.yaml
var kindConfigFile []byte

//go:embed scripts/*
var hackScriptsFS embed.FS

type Cluster struct {
	provider           *cluster.Provider
	kubeconfigFilePath string
	name               string
}

const (
	defaultClusterName              = "kommanderapptest"
	directory_for_kind_hack_scripts = "./tmp-kind-hack-scripts"
)

// CreateCluster creates a new kind cluster with the given name.
func CreateCluster(ctx context.Context, name string) (*Cluster, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var err error
	err = extractEmbededHackScripts()
	if err != nil {
		return nil, err
	}

	kubeconfigFile, err := os.CreateTemp("", "*-kubeconfig")
	if err != nil {
		return nil, err
	}

	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))
	if name == "" {
		name = defaultClusterName
	}

	err = provider.Create(name,
		cluster.CreateWithKubeconfigPath(kubeconfigFile.Name()),
		cluster.CreateWithRawConfig(kindConfigFile),
	)

	if err != nil {
		return nil, err
	}

	// Set KUBECONFIG env so that other tools (kubectl, flux CLI, etc.) can
	// discover the cluster without relying on Kind's ExportKubeConfig which
	// has a known bug in v0.24.0 that can panic or produce "file name too
	// long" errors when resolving the default kubeconfig path.
	os.Setenv("KUBECONFIG", kubeconfigFile.Name())

	return &Cluster{
		provider:           provider,
		kubeconfigFilePath: kubeconfigFile.Name(),
		name:               name,
	}, nil
}

// ListNodeNames lists all nodes in the cluster.
func (c *Cluster) ListNodeNames(ctx context.Context) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	nodes, err := c.provider.ListNodes(c.name)
	if err != nil {
		return nil, err
	}

	nodeNames := make([]string, len(nodes))
	for i, node := range nodes {
		nodeNames[i] = node.String()
	}
	return nodeNames, nil
}

// RunScript runs a script on the given node using `docker exec`.
func (c *Cluster) RunScript(ctx context.Context, nodeName, script string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer apiClient.Close()

	rst, err := apiClient.ContainerExecCreate(context.Background(), nodeName,
		container.ExecOptions{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{script},
		})
	if err != nil {
		return err
	}

	response, err := apiClient.ContainerExecAttach(context.Background(), rst.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	defer response.Close()

	data, err := io.ReadAll(response.Reader)
	if err != nil {
		return err
	}

	log.Println(string(data))

	return nil
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

// KubeconfigForPeers generates a kubeconfig file that can be used by other containers
// on the same Docker network to access this cluster. It replaces the localhost server
// address with the control-plane container name.
func (c *Cluster) KubeconfigForPeers() (string, error) {
	// Read the original kubeconfig
	kubeconfigData, err := os.ReadFile(c.kubeconfigFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	// The control-plane node name follows the pattern: <cluster-name>-control-plane
	controlPlaneHost := fmt.Sprintf("%s-control-plane", c.name)

	// Replace 127.0.0.1 or localhost with the control-plane container name
	// Kind uses port 6443 internally on the control-plane container
	kubeconfigStr := string(kubeconfigData)
	// Match patterns like https://127.0.0.1:XXXXX or https://localhost:XXXXX
	// and replace with the control-plane container address
	kubeconfigStr = replaceServerAddress(kubeconfigStr, controlPlaneHost)

	// Create a temporary file for the peer kubeconfig
	peerKubeconfigFile, err := os.CreateTemp("", "*-peer-kubeconfig")
	if err != nil {
		return "", fmt.Errorf("failed to create peer kubeconfig file: %w", err)
	}
	defer peerKubeconfigFile.Close()

	if _, err := peerKubeconfigFile.WriteString(kubeconfigStr); err != nil {
		return "", fmt.Errorf("failed to write peer kubeconfig: %w", err)
	}

	return peerKubeconfigFile.Name(), nil
}

// replaceServerAddress replaces the server address in kubeconfig with the peer-accessible address.
func replaceServerAddress(kubeconfig, controlPlaneHost string) string {
	// Kind clusters expose the API server on port 6443 inside the container
	// The kubeconfig typically has something like: server: https://127.0.0.1:XXXXX
	// We need to replace it with: server: https://<control-plane-container>:6443

	// Use a simple string replacement approach
	// Find the server line and replace the host:port
	lines := strings.Split(kubeconfig, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "server:") {
			// Replace the server URL with the peer-accessible one
			lines[i] = fmt.Sprintf("    server: https://%s:6443", controlPlaneHost)
		}
	}
	return strings.Join(lines, "\n")
}

// Extracts the embedded files to file system
func extractEmbededHackScripts() error {
	// Create the target directory if it doesn't exist
	err := os.MkdirAll(directory_for_kind_hack_scripts, 0755)
	if err != nil {
		return err
	}

	// Read the embedded files from the "scripts" directory
	entries, err := hackScriptsFS.ReadDir("scripts")
	if err != nil {
		return err
	}

	// Extract each file from embed.FS and write it to the target directory
	for _, entry := range entries {
		data, err := hackScriptsFS.ReadFile("scripts/" + entry.Name())
		if err != nil {
			return err
		}

		// Create the file in the target directory
		targetPath := filepath.Join(directory_for_kind_hack_scripts, entry.Name())
		err = os.WriteFile(targetPath, data, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}
