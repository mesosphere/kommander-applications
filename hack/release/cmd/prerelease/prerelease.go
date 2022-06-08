package prerelease

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/envsubst"
	"github.com/spf13/cobra"
)

var Cmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

const (
	chartVersionFlagName          = "chart-version"
	kommanderApplicationsFlagName = "kommander-applications-repo"
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
			chartVersion := Cmd.Flag(chartVersionFlagName).Value.String()
			kommanderApplicationsRepo := Cmd.Flag(kommanderApplicationsFlagName).Value.String()
			if _, err := os.Stat(kommanderApplicationsRepo); os.IsNotExist(err) {
				return err
			}
			fullChartVersion := fmt.Sprintf(kommanderChartVersionTemplate, chartVersion)
			err := updateChartVersions(kommanderApplicationsRepo, fullChartVersion)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated Kommander chart version to %s", chartVersion)
			return nil
		},
	}
	Cmd.Flags().String(chartVersionFlagName, "", "the new Kommander chart version")
	Cmd.Flags().String(kommanderApplicationsFlagName, "", "the path to the local kommander-applications repository to modify")

	err := Cmd.MarkFlagRequired(chartVersionFlagName)
	if err != nil {
		log.Fatal(err)
	}

	err = Cmd.MarkFlagRequired(kommanderApplicationsFlagName)
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

		// Update the kommanderChartVersion value
		parsedFile, err := envsubst.ParseFile(helmReleaseFilePath)
		subVars := map[string]string{
			"kommanderChartVersion": chartVersion,
			"releaseNamespace":      "${releaseNamespace}",
		}
		updatedFile, err := parsedFile.Execute(func(s string) string {
			return subVars[s]
		})

		if !strings.Contains(updatedFile, chartVersion) {
			return fmt.Errorf("failed to update Kommander HelmRelease chart version")
		}

		err = os.WriteFile(helmReleaseFilePath, []byte(updatedFile), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
