package cmd

import (
	"fmt"
	"sort"

	"github.com/mesosphere/kommander-applications/hack/static/pkg/bloodhound"
	"github.com/mesosphere/kommander-applications/hack/static/pkg/kapps"

	"github.com/mesosphere/dkp-bloodhound/pkg/parse"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/k8sresource"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kommanderapplicationversion"

	"github.com/spf13/cobra"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	syaml "sigs.k8s.io/yaml"
)

var versionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "Output all kommander-applications versions",

	RunE: func(c *cobra.Command, _ []string) error {
		cursor := bloodhound.Run(branch, tag)

		fmt.Println("Getting chart versions for all applications...")
		kApps, err := kapps.List(cursor)
		if err != nil {
			return err
		}

		sort.Sort(kApps)

		k, err := cursor.GetByID("kommander-application://kommander")
		if err != nil {
			return err
		}

		fmt.Println("Determining which applications are installed by default...")
		defaultApps, err := getKommanderDefaultApps(k)
		if err != nil {
			return err
		}

		for i, ka := range kApps {
			if _, ok := defaultApps[ka.AppID]; ok {
				kApps[i].Enabled = true
			}
		}

		outYAML, err := syaml.Marshal(kApps)
		if err != nil {
			return err
		}

		return write(outYAML, outFile)
	},
}

func getKommanderDefaultApps(cursor parse.Cursor) (map[string]struct{}, error) {
	for _, child := range cursor.Children() {
		childNode := child.Node()
		switch childNode.(type) {
		case *kommanderapplicationversion.Node:
			for _, child2 := range child.Children() {
				child2Node := child2.Node()
				switch c2 := child2Node.(type) {
				case *k8sresource.Node:
					if c2.ID().(kyaml.ResourceIdentifier).Kind == "ConfigMap" {
						cmYAML, err := c2.RNode.String()
						if err != nil {
							return nil, err
						}
						valuesYAML, err := traverseYAML(cmYAML, "data", "values.yaml")
						if err != nil {
							return nil, err
						}
						defaultApps, err := traverseYAML(valuesYAML.(string), "attached", "prerequisites", "defaultApps")
						if err != nil {
							return nil, err
						}
						enterpriseApps, err := traverseYAML(valuesYAML.(string), "kommander-licensing", "defaultEnterpriseApps")
						if err != nil {
							return nil, err
						}
						result := make(map[string]struct{})
						for name := range defaultApps.(map[string]interface{}) {
							result[name] = struct{}{}
						}
						for name := range enterpriseApps.(map[string]interface{}) {
							result[name] = struct{}{}
						}
						return result, nil
					}
				default:
				}
			}
		default:
		}
	}
	return make(map[string]struct{}), nil
}

func traverseYAML(yml string, path ...string) (interface{}, error) {
	var nested map[string]interface{}
	if err := syaml.Unmarshal([]byte(yml), &nested); err != nil {
		return nil, err
	}

	for i, k := range path {
		got, ok := nested[k]
		if !ok {
			return nil, fmt.Errorf("failed to find value at %s in %#v", path[0], nested)
		}
		if i == len(path)-1 {
			return got, nil
		}
		switch gt := got.(type) {
		case map[string]interface{}:
			nested = gt
		case string:
			return traverseYAML(gt, path[i+1:]...)
		default:
		}
	}
	return nil, fmt.Errorf("not found")
}
