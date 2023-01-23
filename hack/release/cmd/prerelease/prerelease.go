package prerelease

import (
	"fmt"
	"log"
	"os"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/chartversion"
	"github.com/spf13/cobra"
)

var Cmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

const (
	versionFlagName = "version"
	repoFlagName    = "repo"
)

func init() { //nolint:gochecknoinits // Initializing cobra application.
	Cmd = &cobra.Command{
		Use:   "pre-release",
		Short: "Handles pre-release tasks for kommander-applications (i.e. updating Kommander chart versions)",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartVersion := Cmd.Flag(versionFlagName).Value.String()
			kommanderApplicationsRepo := Cmd.Flag(repoFlagName).Value.String()
			if _, err := os.Stat(kommanderApplicationsRepo); os.IsNotExist(err) {
				return err
			}

			err := chartversion.UpdateChartVersions(kommanderApplicationsRepo, chartVersion)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated Kommander chart version to %s", chartVersion)
			return nil
		},
	}
	Cmd.Flags().String(versionFlagName, "", "the new Kommander chart version")
	Cmd.Flags().String(repoFlagName, "", "the path to the local kommander-applications repository to modify")

	err := Cmd.MarkFlagRequired(versionFlagName)
	if err != nil {
		log.Fatal(err)
	}

	err = Cmd.MarkFlagRequired(repoFlagName)
	if err != nil {
		log.Fatal(err)
	}
}
