package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/mesosphere/kommander-applications/hack/release/cmd/postrelease"
	"github.com/mesosphere/kommander-applications/hack/release/cmd/prerelease"
    "github.com/mesosphere/kommander-applications/hack/release/cmd/applyoci"
)

var rootCmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

func init() { //nolint:gochecknoinits // Initializing cobra application.
	rootCmd = &cobra.Command{}
	rootCmd.AddCommand(prerelease.Cmd)
	rootCmd.AddCommand(postrelease.Cmd)
    rootCmd.AddCommand(applyoci.Cmd)
}

func Execute(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
