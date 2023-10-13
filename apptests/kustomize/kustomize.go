package kustomize

import (
	"github.com/drone/envsubst"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type Kustomize struct {
	directory   string            // The directory where the kustomization file is located
	substitutes map[string]string // The map of environment variables to substitute in the resources
	resources   resmap.ResMap     // The map of resources generated by the kustomization process
}

// New creates a new Kustomize instance with the given directory and substitutes.
func New(dir string, subs map[string]string) *Kustomize {
	return &Kustomize{
		directory:   dir,
		substitutes: subs,
		resources:   resmap.New(),
	}
}

// Build runs the kustomization process on the directory and substitutes the
// environment variables in the resources.
func (k *Kustomize) Build() error {
	opts := krusty.MakeDefaultOptions()
	opts.Reorder = krusty.ReorderOptionLegacy

	kustomizer := krusty.MakeKustomizer(opts)

	// Run the kustomizer on the directory and get the resource map
	resourceMap, err := kustomizer.Run(filesys.MakeFsOnDisk(), k.directory)
	if err != nil {
		return err
	}

	k.resources.Clear()
	for _, r := range resourceMap.Resources() {
		yaml, err := r.AsYAML()
		if err != nil {
			return err
		}

		out, err := envsubst.Eval(string(yaml), func(s string) string {
			return k.substitutes[s]
		})
		if err != nil {
			return err
		}

		// Convert the output string to a resource and append it to the resources map
		res, err := newResourceFromString(out)
		if err != nil {
			return err
		}
		k.resources.Append(res)
	}

	return nil
}

// Output returns the YAML representation of the resources map as a string.
func (k *Kustomize) Output() ([]byte, error) {
	yml, err := k.resources.AsYaml()
	if err != nil {
		return nil, err
	}
	return yml, nil
}

// newResourceFromString converts a given string to a Kubernetes resource.
func newResourceFromString(str string) (*resource.Resource, error) {
	fc := resmap.NewFactory(&resource.Factory{})
	resource, err := fc.NewResMapFromBytes([]byte(str))
	if err != nil {
		return nil, err
	}

	return resource.Resources()[0], nil
}
