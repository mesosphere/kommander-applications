package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mesosphere/kommander-applications/hack/release/utils"
)

const (
	KommanderHelmReleasePathPattern        = "./services/kommander/*/kommander.yaml"
	KommanderAppMgmtHelmReleasePathPattern = "./services/kommander-appmanagement/*/kommander-appmanagement.yaml"
)

// KommanderChartVersion updates the kommander chart version in the kommander-applications repo. It takes the
// kommander-applications repo path and the new kommander chart version.
func KommanderChartVersion(kommanderApplicationsRepo, chartVersion string) error {
	kommanderHelmReleasePaths := []string{KommanderHelmReleasePathPattern, KommanderAppMgmtHelmReleasePathPattern}
	for _, helmReleasePath := range kommanderHelmReleasePaths {
		// Find the HelmRelease
		matches, err := filepath.Glob(filepath.Join(kommanderApplicationsRepo, helmReleasePath))
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("no matches found for HelmRelease path %s (verify the kommander-applications repo path is correct)", helmReleasePath)
		}
		if len(matches) > 1 {
			return fmt.Errorf("found > 1 match for HelmRelease path %s (there should only be one match)", helmReleasePath)
		}
		helmReleaseFilePath := matches[0]

		// Updates the HelmRelease file with given chart version
		updatedFile, err := utils.EvalFile(
			helmReleaseFilePath,
			map[string]string{
				"kommanderChartVersion": chartVersion,
				"releaseNamespace":      "${releaseNamespace}",
			},
		)
		if err != nil {
			return fmt.Errorf("failed to update HelmRelease file %s: %v", helmReleaseFilePath, err)
		}

		// Verify that the HelmRelease file was updated
		if !strings.Contains(updatedFile, chartVersion) {
			return fmt.Errorf("failed to update Kommander HelmRelease chart version")
		}

		// Write the updated HelmRelease file to the same location
		err = os.WriteFile(helmReleaseFilePath, []byte(updatedFile), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
