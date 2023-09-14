package apptests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_findDependencies(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	testdata := filepath.Join(cwd, "testdata")

	dep, err := findDependencies(filepath.Join(testdata, "A"))
	assert.NoError(t, err)
	assert.Equal(t, len(dep), 2)
	assert.Equal(t, dep[0], "B")
	assert.Equal(t, dep[1], "D")
}

func Test_DependencyList(t *testing.T) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	testdata := filepath.Join(cwd, "testdata")

	list, err := DependencyList(filepath.Join(testdata, "A"))
	assert.NoError(t, err)

	// there are 4 dependencies
	assert.Equal(t, list.Len(), 4)
	d1 := list.Front()
	assert.Contains(t, d1.Value.(string), "testdata/C")
	assert.Contains(t, d1.Next().Value.(string), "testdata/D")
	assert.Contains(t, d1.Next().Next().Value.(string), "testdata/B")
	assert.Contains(t, d1.Next().Next().Next().Value.(string), "testdata/A")
}
