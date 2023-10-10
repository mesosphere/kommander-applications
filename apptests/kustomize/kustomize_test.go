package kustomize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuild(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	builder := New(
		filepath.Join(wd, "testdata"),
		map[string]string{
			"name":      "test",
			"namespace": "test",
		},
	)
	err = builder.Build()
	assert.NoError(t, err)

	output, err := builder.Output()
	assert.NoError(t, err)

	// define the expected output
	expected := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: test
`

	assert.Equal(t, output, expected)
}
