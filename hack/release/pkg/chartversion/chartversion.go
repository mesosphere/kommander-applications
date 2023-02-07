package chartversion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/envsubst"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

const kommanderChartVersionTemplate = "${kommanderChartVersion:=%s}"

var (
	kommanderHelmReleasePathPattern        = filepath.Join(constants.KommanderAppPath, "*/kommander.yaml")
	kommanderAppMgmtHelmReleasePathPattern = filepath.Join(constants.KommanderAppMgmtPath, "*/kommander-appmanagement.yaml")
	kommanderOperatorPath                  = "./common/kommander-operator/helmrelease.yaml"
	filesContainingKommanderVersion        = []string{
		kommanderHelmReleasePathPattern,
		kommanderAppMgmtHelmReleasePathPattern,
		kommanderOperatorPath,
	}
)

func UpdateChartVersions(kommanderApplicationsRepo, chartVersion string) error {
	chartVersion = fmt.Sprintf(kommanderChartVersionTemplate, chartVersion)

	for _, helmReleasePath := range filesContainingKommanderVersion {
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

		// Update the kommanderChartVersion value
		parsedFile, err := envsubst.ParseFile(helmReleaseFilePath)
		if err != nil {
			return err
		}
		subVars := map[string]string{
			"kommanderChartVersion": chartVersion,
			"releaseNamespace":      "${releaseNamespace}",
		}
		updatedFile, err := parsedFile.Execute(func(s string) string {
			return subVars[s]
		})
		if err != nil {
			return err
		}

		if !strings.Contains(updatedFile, chartVersion) {
			return fmt.Errorf("failed to update Kommander HelmRelease chart version")
		}

		err = os.WriteFile(helmReleaseFilePath, []byte(updatedFile), 0o644)
		if err != nil {
			return err
		}
	}
	return nil
}
