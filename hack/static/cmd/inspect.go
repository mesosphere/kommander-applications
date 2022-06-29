package cmd

import (
	"bufio"
	"fmt"

	"github.com/mesosphere/kommander-applications/hack/static/pkg/bloodhound"

	"github.com/mesosphere/dkp-bloodhound/pkg/parse"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Output entire dkp-bloodhound node hierarchy for manual analysis",

	RunE: func(_ *cobra.Command, _ []string) error {
		cursor := bloodhound.Run(branch, tag)

		file, err := open(outFile)
		if err != nil {
			return err
		}
		defer file.Close()

		w := bufio.NewWriter(file)
		descNode(w, cursor, "")
		w.Flush()

		return nil
	},
}

func descNode(w *bufio.Writer, cursor parse.Cursor, prefix string) {
	node := cursor.Node()
	if _, isRootNode := node.(parse.RootNode); !isRootNode {
		w.WriteString(fmt.Sprintf("%s%s\n", prefix, node))
		prefix = "  " + prefix
	}
	for _, child := range cursor.Children() {
		descNode(w, child, prefix)
	}
}
