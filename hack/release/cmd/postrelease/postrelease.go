package postrelease

import (
	"fmt"
	"log"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/appversion"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/chartversion"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/extraimages"
	"github.com/mesosphere/kommander-applications/hack/release/pkg/upgradematrix"
	"github.com/spf13/cobra"
)

var Cmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

const (
	versionFlagName = "version"
	repoFlagName    = "repo"
)

func init() { //nolint:gochecknoinits // Initializing cobra application.
	Cmd = &cobra.Command{
		Use:   "post-release",
		Short: "Handles post-release tasks for kommander-applications (i.e. updating Kommander chart versions)",
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			chartVersion, err := semver.NewVersion(Cmd.Flag(versionFlagName).Value.String())
			if err != nil {
				return fmt.Errorf("cannot parse given version: %w", err)
			}
			kommanderApplicationsRepo := Cmd.Flag(repoFlagName).Value.String()
			if _, err := os.Stat(kommanderApplicationsRepo); os.IsNotExist(err) {
				return err
			}

			if err := chartversion.UpdateChartVersions(kommanderApplicationsRepo, chartVersion.Original()); err != nil {
				return err
			}

			if err := appversion.SetKommanderAppsVersion(
				cmd.Context(),
				kommanderApplicationsRepo,
				chartVersionToAppVersion(chartVersion),
			); err != nil {
				return err
			}

			if _, err := appversion.ReplaceContent(
				cmd.Context(),
				kommanderApplicationsRepo,
				chartVersionToAppVersion(chartVersion),
			); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated Kommander chart version to %s", chartVersion)

			if err := upgradematrix.UpdateUpgradeMatrix(
				cmd.Context(),
				kommanderApplicationsRepo,
			); err != nil {
				return err
			}

			if err := extraimages.UpdateExtraImagesVersions(kommanderApplicationsRepo, chartVersion.Original()); err != nil {
				return err
			}

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

func chartVersionToAppVersion(ver *semver.Version) string {
	return fmt.Sprintf("0.%d.%d", ver.Minor(), ver.Patch())
}
