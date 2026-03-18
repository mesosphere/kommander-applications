package manifests

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-getter/v2"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

const nkpArtifactsOutput = "artifacts_full.yaml"

// UpdateArtifactsManifest downloads the NKP CLI via go-getter (version hardcoded), runs
// "nkp validate catalog-repository" in the given repo, and writes artifacts_full.yaml.
func UpdateArtifactsManifest(ctx context.Context, log io.Writer, repo string) error {
	version := constants.NKPCLIVersion
	goos := runtime.GOOS
	goarch := "amd64" // We publish only amd64 binaries today. Make this dynamic if/when we publish arm etc.
	url := fmt.Sprintf("%s/nkp_%s_%s_%s.tar.gz", constants.DefaultSourceNKPBase, version, goos, goarch)

	parentDir, err := os.MkdirTemp("", "nkp-cli-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(parentDir) }()
	// Destination must not exist so go-getter creates it and extracts the .tar.gz into it.
	extractDir := filepath.Join(parentDir, "extract")
	_, err = getter.Get(ctx, extractDir, url)
	if err != nil {
		return fmt.Errorf("download nkp CLI: %w", err)
	}
	binaryPath := filepath.Join(extractDir, "nkp")

	configPath := filepath.Join(repo, ".bloodhound.yml")
	artifactsPath := filepath.Join(repo, nkpArtifactsOutput)
	cmd := exec.CommandContext(ctx, binaryPath, "validate", "catalog-repository",
		"--repo-dir", repo,
		"--config", configPath,
		"--artifacts-output", artifactsPath,
	)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nkp validate catalog-repository: %w", err)
	}
	return nil
}
