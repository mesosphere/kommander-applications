package bloodhound

import (
	"fmt"

	"github.com/mesosphere/dkp-bloodhound/pkg/parse"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/directory"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/fluxkustomization"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/gitrepository"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/gitrepositorypath"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/helm"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/k8sresource"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kommanderapplication"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kommanderapplicationversion"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/kustomizationdir"
	"github.com/mesosphere/dkp-bloodhound/pkg/parse/yaml"
)

func Run(branch, tag string) parse.Cursor {
	substitutionVars := map[string]string{
		"releaseNamespace":       "namespace",
		"workspaceNamespace":     "namespace",
		"certificatesIssuerName": "issuer",
	}

	parser := parse.NewMultiParser(
		&yaml.Parser{},
		&gitrepository.Parser{},
		&fluxkustomization.Parser{},
		&helm.Parser{},
		&k8sresource.Parser{},
		&gitrepositorypath.Parser{},
		&kommanderapplication.Parser{},
		&kommanderapplicationversion.Parser{SubstitutionVars: substitutionVars},
		&kustomizationdir.Parser{SubstitutionVars: substitutionVars},
		&directory.Parser{},
	)

	repoNamespace := "kommander-flux"
	repoName := "management"
	repoURL := "https://github.com/mesosphere/kommander-applications.git"

	var repoYAML yaml.YAMLString
	if tag != "" {
		repoYAML = gitrepository.YAMLWithTag(repoNamespace, repoName, repoURL, tag)
		fmt.Printf("Analyzing kommander-applications repo on tag %s...\n", tag)
	} else {
		repoYAML = gitrepository.YAMLWithBranch(repoNamespace, repoName, repoURL, branch)
		fmt.Printf("Analyzing kommander-applications repo on branch %s...\n", branch)
	}

	cursor := parse.NewCursorWithEmptyGraph()
	parser.ParseSequential(cursor,
		repoYAML,
		substitutionVarConfigMap(substitutionVars),
		gitrepositorypath.Input(repoNamespace, repoName, "common/base"),
		gitrepositorypath.Input(repoNamespace, repoName, "services"),
	)
	gitrepository.CleanupTempDirectories(cursor)

	return cursor
}

func substitutionVarConfigMap(vars map[string]string) yaml.YAMLString {
	manifest := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: substitution-vars
  namespace: ` + vars["releaseNamespace"] + `
data:
`
	for k, v := range vars {
		manifest += fmt.Sprintf("  %s: %s\n", k, v)
	}
	return yaml.YAMLString(manifest)
}
