package apptests

import (
	"container/list"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/yaml"
)

type DeploymentTest struct {
	path         string // the path to directory contains service
	dependencies *list.List
}

func (d *DeploymentTest) RunDeployment() error {
	// load dependencies
	err := d.UpdateDependencyList()
	if err != nil {
		return err
	}
	// provision environment
	// install dependencies
	// Assert that each dependency to be come ready
	// - flux kustomizations
	// send out proper logs

	return nil
}

// UpdateDependencyList contains all the prerequisites that the current application needs to have deployed
// before it can be deployed.
func (d *DeploymentTest) UpdateDependencyList() error {
	tmpList := list.New() // a temporary list to traverse dependency list
	tmpList.PushFront(d.path)
	seen := make(map[string]bool, tmpList.Len())
	finalList := list.New() // final dependency list

	for tmpList.Len() > 0 {
		dep := tmpList.Front()
		if dep == nil {
			break
		}
		if _, ok := seen[dep.Value.(string)]; ok {
			tmpList.Remove(dep)
			continue
		}

		path := dep.Value.(string)
		// put the dependency into the final dependency list
		// newer dependencies are added to the front of the list
		finalList.PushFront(path)

		// find all dependencies of this dependency
		// and add them to the end of temporary list
		dependencies, err := findDependencyList(
			filepath.Join(path),
		)
		if err != nil {
			return err
		}
		for _, v := range dependencies {
			tmpList.PushBack(filepath.Join(path, "..", v))
		}

		// remove the dependency from the temporary list
		// and make sure it won't be processed anymore
		tmpList.Remove(dep)
		seen[dep.Value.(string)] = true
	}
	d.dependencies = finalList

	return nil
}

// findDependencyList returns the list of the declared dependencies in the given path.
func findDependencyList(path string) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(path, "metadata.yaml"))
	if err != nil {
		return nil, err
	}

	type metadataFile struct {
		// RequiredDependencies []string // TODO: will be implemented
		Dependencies []string
	}
	meta := metadataFile{}
	err = yaml.Unmarshal(content, &meta)
	if err != nil {
		return nil, err
	}

	return meta.Dependencies, nil
}
