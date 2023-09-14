package apptests

import (
	"container/list"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// DependencyList returns all dependencies for the current application
// in correct order that needs to have deployed before it can be deployed.
//
// The function uses two lists, `pending` and `ordered`, to track the dependencies.
// A new discovered dependency is added to the end of pending list if it is not seen before.
// Dependencies are processed in the pending list, until it gets empty.
//
// If a dependency in the pending list is new and has other dependencies, those
// dependencies are added to the end of the pending list.
// The function uses a map to store seen dependencies and avoid circular dependency.
func DependencyList(applicationPath string) (*list.List, error) {
	pending := list.New() // a temporary list to traverse dependency list
	pending.PushFront(applicationPath)

	seen := make(map[string]bool, pending.Len()) // a map to store seen dependencies
	ordered := list.New()                        // final dependency list

	for pending.Len() > 0 {
		// returns the first element of list.
		item := pending.Front()
		if item == nil {
			break
		}
		if _, ok := seen[item.Value.(string)]; ok {
			pending.Remove(item)
			continue
		}

		path := item.Value.(string)
		// prepend the dependency to the ordered list
		ordered.PushFront(path)

		// find all dependencies of this dependency
		// and append them to pending list
		dependencies, err := findDependencies(
			filepath.Join(path),
		)
		if err != nil {
			return nil, err
		}
		for _, d := range dependencies {
			pending.PushBack(filepath.Join(path, "..", d))
		}

		// remove the processed dependency from the pending list and mark it as seen
		pending.Remove(item)
		seen[item.Value.(string)] = true
	}

	return ordered, nil
}

// findDependencies returns the list of the declared dependencies in the given path.
func findDependencies(path string) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(path, "metadata.yaml"))
	if err != nil {
		return nil, err
	}

	type metadataFile struct {
		Dependencies []string `yaml:"dependencies"`
	}
	meta := metadataFile{}
	err = yaml.Unmarshal(content, &meta)
	if err != nil {
		return nil, err
	}

	return meta.Dependencies, nil
}
