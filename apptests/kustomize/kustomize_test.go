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
			"name":      "test-name",
			"namespace": "kommander",
		},
	)
	err = builder.Build()
	assert.NoError(t, err)

	output, err := builder.Output()
	assert.NoError(t, err)

	// define the expected output
	expected := `apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: HelmRepository
metadata:
  name: test-name
  namespace: kommander
spec:
  interval: 10m
  timeout: 1m
  url: https://charts.bitnami.com/bitnami/
`

	assert.Equal(t, output, expected)
}
