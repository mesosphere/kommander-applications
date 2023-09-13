package apptests

import (
	"container/list"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findDependencyList(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	testdata := filepath.Join(cwd, "testdata")

	dep, err := findDependencyList(filepath.Join(testdata, "A"))
	assert.NoError(t, err)
	assert.Equal(t, len(dep), 1)
	assert.Equal(t, dep[0], "B")
}

func Test_UpdateDependencyList(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	testdata := filepath.Join(cwd, "testdata")

	d := &DeploymentTest{
		path:         filepath.Join(testdata, "A"),
		dependencies: list.New(),
	}
	err = d.UpdateDependencyList()
	assert.NoError(t, err)

	// there are 4 dependencies
	assert.Equal(t, d.dependencies.Len(), 4)
	d1 := d.dependencies.Front()
	assert.Contains(t, d1.Value.(string), "testdata/C")
	assert.Contains(t, d1.Next().Value.(string), "testdata/D")
	assert.Contains(t, d1.Next().Next().Value.(string), "testdata/B")
	assert.Contains(t, d1.Next().Next().Next().Value.(string), "testdata/A")
}
