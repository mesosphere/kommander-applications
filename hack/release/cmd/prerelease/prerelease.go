package prerelease

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mesosphere/kommander-applications/hack/release/utils"
)

var Cmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

const (
	versionFlagName               = "version"
	kommanderChartVersionTemplate = "${kommanderChartVersion:=%s}"

	kommanderHelmReleasePathPattern        = "./services/kommander/*/kommander.yaml"
	kommanderAppMgmtHelmReleasePathPattern = "./services/kommander-appmanagement/*/kommander-appmanagement.yaml"
)

func init() { //nolint:gochecknoinits // Initializing cobra application.
	Cmd = &cobra.Command{
		Use:   "pre-release",
		Short: "Handles pre-release tasks for kommander-applications (i.e. updating Kommander chart versions)",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartVersion := Cmd.Flag(versionFlagName).Value.String()
			fullChartVersion := fmt.Sprintf(kommanderChartVersionTemplate, chartVersion)

			// Get the kommander-applications repo path
			rootDir, err := utils.GetRootDir()
			if err != nil {
				return fmt.Errorf("failed to get root directory: %v", err)
			}

			// Update the kommanderChartVersion value
			err = updateChartVersions(rootDir, fullChartVersion)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated Kommander chart version to %s", chartVersion)

			return nil
		},
	}
	Cmd.Flags().String(versionFlagName, "", "the new Kommander chart version")

	err := Cmd.MarkFlagRequired(versionFlagName)
	if err != nil {
		log.Fatal(err)
	}
}

func updateChartVersions(kommanderApplicationsRepo, chartVersion string) error {
	kommanderHelmReleasePaths := []string{kommanderHelmReleasePathPattern, kommanderAppMgmtHelmReleasePathPattern}
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
