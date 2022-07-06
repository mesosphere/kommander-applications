package kapps

import (
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/mesosphere/dkp-bloodhound/pkg/parse"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/fluxkustomization"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/helm"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/k8sresource"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kommanderapplication"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kommanderapplicationversion"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kustomizationdir"

	fluxsourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	syaml "sigs.k8s.io/yaml"
)

type KApp struct {
	AppID,
	AppName,
	DisplayName,
	Type,
	Path,
	Repo,
	Chart,
	Version,
	AppVersion,
	LatestVersion string
	Enabled      bool
	Metadata     map[string]Meta
	ValuesYAML   string              `json:"-" yaml:"-"`
	Dependencies []*chart.Dependency `json:"-" yaml:"-"`
}

type Meta struct {
	Location string
	Data     interface{}
}

type KApps []*KApp

func (k KApps) Len() int {
	return len(k)
}

func (k KApps) Swap(i, j int) {
	k[i], k[j] = k[j], k[i]
}

func (k KApps) Less(i, j int) bool {
	return strings.Compare(k[i].AppID, k[j].AppID) < 1
}

func List(cursor parse.Cursor) (KApps, error) {
	var repos []*repo.Entry
	var kApps []*KApp
	var err error

	for _, child := range cursor.Children() {
		childNode := child.Node()
		switch cn := childNode.(type) {
		case *kustomizationdir.Node:
			repos, err = getHelmRepoEntries(child)
			if err != nil {
				return nil, err
			}
		case *kommanderapplication.Node:
			if ka := getKommanderAppInfo(cn, child); ka != nil {
				kApps = append(kApps, ka)
			}
		default:
		}
	}

	index := search.NewIndex()
	for _, re := range repos {
		u, err := url.Parse(re.URL)
		if err != nil {
			return nil, err
		}
		u.Path = path.Join(u.Path, "index.yaml")
		indexFileURL := u.String()
		data, err := get(&http.Client{Timeout: time.Second * 5}, indexFileURL)
		if err != nil {
			return nil, err
		}
		indexFile := &repo.IndexFile{}
		if err := syaml.Unmarshal(data, indexFile); err != nil {
			return nil, err
		}
		index.AddRepo(re.Name, indexFile, true)
	}

	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	actionConfig := &action.Configuration{
		RegistryClient: registryClient,
	}
	valuesClient := action.NewShowWithConfig(action.ShowValues, actionConfig)
	settings := cli.New()

	for i, ka := range kApps {
		chartID := ka.Repo + "/" + ka.Chart
		results, err := index.Search(chartID, 25, false)
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			continue
		}
		search.SortScore(results)
		r := results[0]
		meta := r.Chart.Metadata

		if meta.Name != ka.Chart {
			continue
		}

		if meta.AppVersion == "" {
			kApps[i].AppVersion = meta.Version
		} else {
			kApps[i].AppVersion = meta.AppVersion
		}
		kApps[i].LatestVersion = meta.Version

		if ka.AppID != "kube-prometheus-stack" {
			continue
		}

		chartURL := r.Chart.URLs[0]
		chartPath, err := valuesClient.ChartPathOptions.LocateChart(chartURL, settings)
		if err != nil {
			return nil, err
		}
		values, err := valuesClient.Run(chartPath)
		if err != nil {
			return nil, err
		}
		ka.ValuesYAML = values
		ka.Dependencies = meta.Dependencies
	}

	return kApps, nil
}

func getHelmRepoEntries(cursor parse.Cursor) ([]*repo.Entry, error) {
	var helmRepos []*repo.Entry
	for _, child := range cursor.Children() {
		node := child.Node()
		switch nodeType := node.(type) {
		case *k8sresource.Node:
			helmrepoYAML, err := nodeType.RNode.String()
			if err != nil {
				return nil, err
			}
			var helmrepo fluxsourcev1beta2.HelmRepository
			if err := syaml.Unmarshal([]byte(helmrepoYAML), &helmrepo); err != nil {
				return nil, err
			}
			helmRepos = append(helmRepos, &repo.Entry{
				Name: helmrepo.ObjectMeta.Name,
				URL:  helmrepo.Spec.URL,
			})
		default:
		}
	}
	return helmRepos, nil
}

func getKommanderAppInfo(kaNode *kommanderapplication.Node, cursor parse.Cursor) *KApp {
	for _, child := range cursor.Children() {
		childNode := child.Node()
		switch childNode.(type) {
		case *kommanderapplicationversion.Node:
			for _, child2 := range child.Children() {
				child2Node := child2.Node()
				switch c2 := child2Node.(type) {
				case *fluxkustomization.Node:
					for _, child3 := range child2.Children() {
						child3Node := child3.Node()
						switch c3 := child3Node.(type) {
						case *helm.Node:
							return &KApp{
								AppID:       kaNode.AppID,
								AppName:     kaNode.AppID + "-" + c3.HelmRelease.Spec.Chart.Spec.Version,
								DisplayName: kaNode.MetaData.DisplayName,
								Type:        kaNode.MetaData.Type,
								Path:        kaNode.Path[strings.Index(kaNode.Path, "/services"):],
								Repo:        c3.HelmRelease.Spec.Chart.Spec.SourceRef.Name,
								Chart:       c3.HelmRelease.Spec.Chart.Spec.Chart,
								Version:     c3.HelmRelease.Spec.Chart.Spec.Version,
							}
						}
					}
				case *helm.Node:
					return &KApp{
						AppID:       kaNode.AppID,
						AppName:     kaNode.AppID + "-" + c2.HelmRelease.Spec.Chart.Spec.Version,
						DisplayName: kaNode.MetaData.DisplayName,
						Type:        kaNode.MetaData.Type,
						Path:        kaNode.Path[strings.Index(kaNode.Path, "/services"):],
						Repo:        c2.HelmRelease.Spec.Chart.Spec.SourceRef.Name,
						Chart:       c2.HelmRelease.Spec.Chart.Spec.Chart,
						Version:     c2.HelmRelease.Spec.Chart.Spec.Version,
					}
				default:
				}
			}
		default:
		}
	}
	return nil
}

func get(c *http.Client, url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}
