package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/mesosphere/dkp-cli-runtime/core/cmd/root"
	"github.com/spf13/cobra"
)

var (
	tag     string
	branch  string
	outFile string
	rootCmd *cobra.Command
)

func init() {
	var opts *root.RootOptions
	rootCmd, opts = root.NewCommand(os.Stdout, os.Stderr)
	rootCmd.SetErr(opts.Output.ErrorWriter())
	rootCmd.TraverseChildren = true

	rootCmd.PersistentFlags().StringVarP(&tag, "tag", "t", "", "The tag to analyze")
	rootCmd.PersistentFlags().StringVarP(&branch, "branch", "b", "main", "The branch to analyze if 'tag' is not set")
	rootCmd.PersistentFlags().StringVarP(&outFile, "output-file", "o", "/dev/stdout", "Where to write output")

	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(versionsCmd)
	rootCmd.AddCommand(bumpsCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("Wrote to %s.\n", outFile)
	}
}

func write(data []byte, path string) error {
	return os.WriteFile(path, data, 0666)
}
