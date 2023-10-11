package cmd

import (
	"fmt"

	"github.com/mesosphere/kommander-applications/apptests/cli"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
	"github.com/spf13/cobra"
)

// settings manages the settings and flags for the command.
var settings = cli.New()

// NewCommand creates and returns the root command for application specific testings.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apptest --applications=app1,app2,app3,...",
		Short: "A CLI tool for applications specific testing",
		Long: `The apptest is a CLI tool that allows you to run tests on different applications.
You can specify which applications you want to test using the --applications flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			apps := settings.Applications
			if len(apps) == 0 {
				return fmt.Errorf("at leas one application must be specified")
			}

			for _, app := range apps {
				sc := scenarios.Get(app)
				if sc == nil {
					// error
				}

				// prepare environment
				env := &environment.Env{}
				err := env.Provision(ctx)
				if err != nil {
					return err
				}

				// execute the scenario
				err = sc.Execute(ctx, env)
				if err != nil {
					return err
				}

				// tear down the environment
				err = env.Destroy(ctx)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	flags := cmd.PersistentFlags()
	settings.AddFlags(flags)

	return cmd
}
