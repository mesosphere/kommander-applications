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

func open(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if os.IsNotExist(err) {
		file, err = os.Create(path)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return file, nil
}

func write(data []byte, path string) error {
	file, err := open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return err
	}
	return nil
}
