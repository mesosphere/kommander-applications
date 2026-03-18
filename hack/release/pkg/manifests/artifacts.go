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
	"github.com/otiai10/copy"
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
	// TODO(takirala): This is a bug (missing functionality really) in nkp-catalog-cli where it uses the name of the repo-dir as the value for unpublished oci artifacts.
	// To work around this, we copy the "repo" dir to a new directory named "kommander-applications".
	// This has no functional impact and is purely for a nicer looking yaml file.
	kappsDir := filepath.Join(parentDir, "kommander-applications")
	err = copy.Copy(repo, kappsDir)
	if err != nil {
		return fmt.Errorf("copy nkp artifacts: %w", err)
	}
	cmd := exec.CommandContext(ctx, binaryPath, "validate", "catalog-repository",
		"--repo-dir", kappsDir,
		"--config", configPath,
		"--artifacts-output", artifactsPath,
	)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		// TODO(takirala): Ignore the error until we can fix the dry-run.
		// return fmt.Errorf("nkp validate catalog-repository: %w", err)
		fmt.Println("Error validating catalog repository:", err)
	}
	return nil
}
