package cmd

import (
	"fmt"

	"github.com/mesosphere/kommander-applications/hack/static/pkg/bloodhound"
	"github.com/mesosphere/kommander-applications/hack/static/pkg/kapps"

	"github.com/spf13/cobra"
	syaml "sigs.k8s.io/yaml"
)

var bumpsCmd = &cobra.Command{
	Use:   "bumps",
	Short: "Output all kommander-applications upstream chart bumps",

	RunE: func(_ *cobra.Command, _ []string) error {
		cursor := bloodhound.Run(branch, tag)

		fmt.Println("Checking upstream charts for new versions...")
		kApps, err := kapps.List(cursor)
		if err != nil {
			return err
		}

		var updates []*kapps.KApp
		for _, ka := range kApps {
			if ka.Version != ka.LatestVersion {
				updates = append(updates, ka)
			}
		}

		outYAML, err := syaml.Marshal(updates)
		if err != nil {
			return err
		}

		return write(outYAML, outFile)
	},
}
