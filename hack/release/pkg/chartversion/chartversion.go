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
	kommanderHelmReleasePathPattern         = filepath.Join(constants.KommanderAppPath, "*/helmrelease/kommander.yaml")
	kommanderAppMgmtHelmReleasePathPattern  = filepath.Join(constants.KommanderAppMgmtPath, "*/helmrelease/kommander-appmanagement.yaml")
	kommanderOperatorDefaultsCMPath         = "./common/kommander-operator/cm.yaml"
	managementPlaneManifestsPath            = "./common/management-plane/manifests/all.yaml"
	nkpclusterManifestsPath                = "./common/nkpcluster/manifests/all.yaml"
	filesContainingKommanderVersion        = []string{
		kommanderHelmReleasePathPattern,
		kommanderAppMgmtHelmReleasePathPattern,
		kommanderOperatorDefaultsCMPath,
		managementPlaneManifestsPath,
		nkpclusterManifestsPath,
	}
)

func UpdateChartVersions(kommanderApplicationsRepo, chartVersion string) error {
	chartVersion = fmt.Sprintf(kommanderChartVersionTemplate, chartVersion)

	for _, filePath := range filesContainingKommanderVersion {
		matches, err := filepath.Glob(filepath.Join(kommanderApplicationsRepo, filePath))
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("no matches found for path %s (verify the kommander-applications repo path is correct)", filePath)
		}
		if len(matches) > 1 {
			return fmt.Errorf("found > 1 match for path %s (there should only be one match)", filePath)
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
			"releaseName":           "${releaseName}",
			"appVersion":            "${appVersion}",
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
