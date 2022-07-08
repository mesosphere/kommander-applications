package cmd

import (
	"fmt"
	"sort"

	"github.com/mesosphere/kommander-applications/hack/static/pkg/bloodhound"
	"github.com/mesosphere/kommander-applications/hack/static/pkg/kapps"

	"github.com/spf13/cobra"
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

		for _, ka := range kApps {
			if ka.AppID == "kube-prometheus-stack" {
				ka.Metadata, err = parseKPSValues(ka.ValuesYAML)
				if err != nil {
					return err
				}
				for _, dep := range ka.Dependencies {
					if dep.Name == "grafana" {
						ka.Metadata["grafana"] = kapps.Meta{
							Location: "[chart.metadata].dependencies",
							Data:     dep.Version,
						}
					}
				}
			}
		}

		outYAML, err := syaml.Marshal(kApps)
		if err != nil {
			return err
		}

		return write(outYAML, outFile)
	},
}

func parseKPSValues(yml string) (map[string]kapps.Meta, error) {
	values := make(map[string]kapps.Meta)

	prometheusVersion, err := traverseYAML(yml, "prometheus", "prometheusSpec", "image", "tag")
	if err != nil {
		return nil, err
	}
	values["prometheus"] = kapps.Meta{
		Location: "[chart.values].prometheus.prometheusSpec.image.tag",
		Data:     prometheusVersion,
	}

	alertmanagerVersion, err := traverseYAML(yml, "alertmanager", "alertmanagerSpec", "image", "tag")
	if err != nil {
		return nil, err
	}
	values["prometheus-alertmanager"] = kapps.Meta{
		Location: "[chart.values].alertmanager.alertmanagerSpec.image.tag",
		Data:     alertmanagerVersion,
	}

	operatorVersion, err := traverseYAML(yml, "prometheusOperator", "image", "tag")
	if err != nil {
		return nil, err
	}
	values["prometheus-operator"] = kapps.Meta{
		Location: "[chart.values].prometheusOperator.image.tag",
		Data:     operatorVersion,
	}

	return values, nil
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
