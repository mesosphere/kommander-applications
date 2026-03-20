package chartversion

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fluxcd/pkg/envsubst"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
)

// kommanderChartVersionDefaultRegex extracts the default value from ${kommanderChartVersion:=v2.18.0-dev}.
var kommanderChartVersionDefaultRegex = regexp.MustCompile(`\$\{kommanderChartVersion:=([^}]+)\}`)

const kommanderChartVersionTemplate = "${kommanderChartVersion:=%s}"

var (
	kommanderHelmReleasePathPattern        = filepath.Join(constants.KommanderAppPath, "*/helmrelease/kommander.yaml")
	kommanderAppMgmtHelmReleasePathPattern = filepath.Join(constants.KommanderAppMgmtPath, "*/helmrelease/kommander-appmanagement.yaml")
	helmReleaseFiles                       = []string{
		kommanderHelmReleasePathPattern,
		kommanderAppMgmtHelmReleasePathPattern,
	}

	kommanderOperatorDefaultsCMPath         = "./common/kommander-operator/manifests/cm.yaml"
	managementOperatorsKustomizationPattern = "./common/*/flux-kustomization.yaml"
	managementOperatorsManifestsPattern     = "./common/*/manifests/all.yaml"
	commonFiles                             = []string{
		kommanderOperatorDefaultsCMPath,
		managementOperatorsKustomizationPattern,
		managementOperatorsManifestsPattern,
	}
)

// GetKommanderChartVersion extracts the current kommander chart version (e.g. v2.18.0-dev)
// from the repo by reading files that contain ${kommanderChartVersion:=version}.
func GetKommanderChartVersion(kommanderApplicationsRepo string) (string, error) {
	cmPath := filepath.Join(kommanderApplicationsRepo, kommanderOperatorDefaultsCMPath)
	data, err := os.ReadFile(cmPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", cmPath, err)
	}
	matches := kommanderChartVersionDefaultRegex.FindStringSubmatch(string(data))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find kommanderChartVersion default in %s", cmPath)
	}
	return matches[1], nil
}

func UpdateChartVersions(kommanderApplicationsRepo, chartVersion string) error {
	chartVersion = fmt.Sprintf(kommanderChartVersionTemplate, chartVersion)

	helmReleaseSubVars := map[string]string{
		"kommanderChartVersion": chartVersion,
		"releaseNamespace":      "${releaseNamespace}",
		"releaseName":           "${releaseName}",
		"appVersion":            "${appVersion}",
	}
	commonSubVars := map[string]string{
		"kommanderChartVersion": chartVersion,
	}

	for _, helmReleasePath := range helmReleaseFiles {
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

		if err = replaceKommanderVersion(helmReleaseFilePath, helmReleaseSubVars); err != nil {
			return err
		}
	}

	for _, filePathPattern := range commonFiles {
		paths, err := filepath.Glob(filepath.Join(kommanderApplicationsRepo, filePathPattern))
		if err != nil {
			return err
		}
		for _, filePath := range paths {
			if err = replaceKommanderVersion(filePath, commonSubVars); err != nil {
				return err
			}
		}
	}

	return nil
}

func replaceKommanderVersion(filePath string, subVars map[string]string) error {
	parsedFile, err := envsubst.ParseFile(filePath)
	if err != nil {
		return err
	}
	updatedFile, err := parsedFile.Execute(func(s string) (string, bool) {
		// Match prior drone/envsubst behavior: unset keys expand to empty string.
		return subVars[s], true
	})
	if err != nil {
		return err
	}

	if !strings.Contains(updatedFile, subVars["kommanderChartVersion"]) {
		return fmt.Errorf("failed to update Kommander chart version in %s", filePath)
	}

	if err = os.WriteFile(filePath, []byte(updatedFile), 0o644); err != nil {
		return err
	}

	return nil
}
