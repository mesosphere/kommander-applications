package cmd

import (
	"fmt"
	"io"

	"github.com/mesosphere/kommander-applications/apptests/appscenarios"
	"github.com/mesosphere/kommander-applications/apptests/cli"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/spf13/cobra"
)

// settings manages the settings and flags for the command.
var settings = cli.New()

// NewCommand creates and returns the root command for application specific testings.
func NewCommand(output io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   "apptest --applications=app1,app2,app3,...",
		Short: "A CLI tool for applications specific testing",
		Long: `The apptest is a CLI tool that allows you to run tests on different applications.
You can specify which applications you want to test using the --applications flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apps := settings.Applications
			if len(apps) == 0 {
				return fmt.Errorf("at leas one application must be specified")
			}

			// validate the given applications
			for _, app := range apps {
				if !appscenarios.Has(app) {
					return fmt.Errorf("could not find app: %s", app)
				}
				// make sure application tests scenario has already been implemented
				appScenario := appscenarios.Get(app)
				if appScenario == nil {
					return fmt.Errorf("test scenario has not been implemented for app:%s", app)
				}
			}

			ctx := cmd.Context()
			for _, app := range apps {
				// prepare environment
				env := &environment.Env{}
				err := env.Provision(ctx)
				if err != nil {
					return fmt.Errorf("could not provision environment for app %s:%w", app, err)
				}
				defer env.Destroy(ctx)

				// run the associated scenario with application
				appScenario := appscenarios.Get(app)
				err = appScenario.Execute(ctx, env)
				if err != nil {
					return fmt.Errorf("test scenario failed for app %s:%w", app, err)
				}

			}

			output.Write([]byte("✓ All tests passed successfully\n"))
			return nil
		},
	}

	flags := cmd.PersistentFlags()
	settings.AddFlags(flags)

	return cmd
}
