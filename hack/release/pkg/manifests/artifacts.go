package manifests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

const nkpArtifactsOutput = "artifacts_full.yaml"

// UpdateArtifactsManifest updates artifacts_full.yaml by replacing:
// - currentVersion (e.g. v2.18.0-dev) with tagVersion in kommander/kommander-appmanagement image refs
// - CAPIMateDefaultVersion (v0.0.0-dev.0) with tagVersion in capimate image refs
// currentVersion should be the version discovered from the repo before any updates
// (e.g. via chartversion.GetKommanderChartVersion).
func UpdateArtifactsManifest(repo, currentVersion, tagVersion string) error {
	artifactsPath := filepath.Join(repo, nkpArtifactsOutput)
	data, err := os.ReadFile(artifactsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", artifactsPath, err)
	}

	content := string(data)
	updated := strings.ReplaceAll(content, currentVersion, tagVersion)
	updated = strings.ReplaceAll(updated, constants.CAPIMateDefaultVersion, tagVersion)
	if updated == content {
		return nil
	}

	if err := os.WriteFile(artifactsPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", artifactsPath, err)
	}

	return nil
}
