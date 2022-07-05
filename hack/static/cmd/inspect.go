package cmd

import (
	"bytes"
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
		buf := bytes.NewBuffer([]byte{})
		descNode(buf, cursor, "")
		return write(buf.Bytes(), outFile)
	},
}

func descNode(buf *bytes.Buffer, cursor parse.Cursor, prefix string) {
	node := cursor.Node()
	if _, isRootNode := node.(parse.RootNode); !isRootNode {
		buf.WriteString(fmt.Sprintf("%s%s\n", prefix, node))
		prefix = "  " + prefix
	}
	for _, child := range cursor.Children() {
		descNode(buf, child, prefix)
	}
}
