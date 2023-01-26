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
	kommanderHelmReleasePathPattern = filepath.Join(constants.KommanderAppPath, "*/kommander.yaml")
	kommanderCMPathPattern          = filepath.Join(
		constants.KommanderAppPath, "*/defaults/cm.yaml")
	kommanderAppMgmtHelmReleasePathPattern = filepath.Join(
		constants.KommanderAppMgmtPath, "*/kommander-appmanagement.yaml")
)

func UpdateChartVersions(kommanderApplicationsRepo, chartVersion string) error {
	chartVersionStr := fmt.Sprintf(kommanderChartVersionTemplate, chartVersion)

	paths := []string{kommanderHelmReleasePathPattern, kommanderAppMgmtHelmReleasePathPattern, kommanderCMPathPattern}
	for _, path := range paths {
		// Find the file
		matches, err := filepath.Glob(filepath.Join(kommanderApplicationsRepo, path))
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("no matches found for path %s (verify the kommander-applications repo path is correct)", path)
		}
		if len(matches) > 1 {
			return fmt.Errorf("found > 1 match for path %s (there should only be one match)", path)
		}
		filePath := matches[0]

		// Update the kommanderChartVersion value
		parsedFile, err := envsubst.ParseFile(filePath)
		if err != nil {
			return err
		}
		subVars := map[string]string{
			"kommanderChartVersion":                              chartVersionStr,
			"releaseNamespace":                                   "${releaseNamespace}",
			"airgappedEnabled":                                   "${airgappedEnabled}",
			"kommanderAuthorizedlisterImageTag":                  "${kommanderAuthorizedlisterImageTag}",
			"kommanderAuthorizedlisterImageRepository":           "${kommanderAuthorizedlisterImageRepository}",
			"certificatesCAIssuerName":                           "${certificatesCAIssuerName}",
			"certificatesIssuerName":                             "${certificatesIssuerName}",
			"certificateIssuerKind":                              "${certificateIssuerKind:-Issuer}",
			"caSecretName":                                       "${caSecretName}",
			"caSecretNamespace":                                  "${caSecretNamespace}",
			"kommanderControllerManagerImageTag":                 "${kommanderControllerManagerImageTag}",
			"kommanderControllerManagerImageRepository":          "${kommanderControllerManagerImageRepository}",
			"kommanderFluxNamespace":                             "${kommanderFluxNamespace}",
			"kommanderGitCredentialsSecretName":                  "${kommanderGitCredentialsSecretName}",
			"ageEncryptionSecretName":                            "${ageEncryptionSecretName}",
			"ageEncryptionSecretKey":                             "${ageEncryptionSecretKey}",
			"kommanderControllerWebhookImageTag":                 "${kommanderControllerWebhookImageTag}",
			"kommanderControllerWebhookImageRepository":          "${kommanderControllerWebhookImageRepository}",
			"kommanderFluxOperatorManagerImageTag":               "${kommanderFluxOperatorManagerImageTag}",
			"kommanderFluxOperatorManagerImageRepository":        "${kommanderFluxOperatorManagerImageRepository}",
			"kommanderLicensingControllerManagerImageTag":        "${kommanderLicensingControllerManagerImageTag}",
			"kommanderLicensingControllerManagerImageRepository": "${kommanderLicensingControllerManagerImageRepository}",
			"kommanderLicensingControllerWebhookImageTag":        "${kommanderLicensingControllerWebhookImageTag}",
			"kommanderLicensingControllerWebhookImageRepository": "${kommanderLicensingControllerWebhookImageRepository}",
			"workspaceNamespace":                                 "${workspaceNamespace}",
		}
		updatedFile, err := parsedFile.Execute(func(s string) string {
			return subVars[s]
		})
		if err != nil {
			return err
		}

		if !strings.Contains(updatedFile, chartVersion) {
			return fmt.Errorf("failed to update Kommander chart version")
		}

		err = os.WriteFile(filePath, []byte(updatedFile), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
