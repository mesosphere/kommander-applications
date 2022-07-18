package prerelease

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/mesosphere/kommander-applications/hack/release/update"
	"github.com/mesosphere/kommander-applications/hack/release/utils"
)

var Cmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

const (
	versionFlagName               = "version"
	kommanderChartVersionTemplate = "${kommanderChartVersion:=%s}"
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
			err = update.KommanderChartVersion(rootDir, fullChartVersion)
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
