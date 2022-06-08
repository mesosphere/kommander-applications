package prerelease

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/drone/envsubst"
	"github.com/fluxcd/helm-controller/api/v2beta1"
	cp "github.com/otiai10/copy"
	"github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

const rootDir = "../../../../"

func TestUpdateChartVersionsSuccessfully(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := fmt.Sprintf(kommanderChartVersionTemplate, "v1.0.0")
	err = updateChartVersions(tmpDir, updateToVersion)
	assert.Nil(t, err)

	kommanderHelmReleasePaths := []string{kommanderHelmReleasePathPattern, kommanderAppMgmtHelmReleasePathPattern}
	for _, helmReleasePath := range kommanderHelmReleasePaths {
		// Get Kommander HR files from the current repo state to validate that changes to the Kommander HelmReleases
		// are compatible with what this tool expects
		beforeUpdateFiles, err := filepath.Glob(filepath.Join(rootDir, helmReleasePath))
		assert.Nil(t, err)

		// Get the tmp Kommander HR files after the chart version has been updated
		afterUpdateFiles, err := filepath.Glob(filepath.Join(tmpDir, helmReleasePath))
		assert.Nil(t, err)

		beforeFile, err := os.ReadFile(beforeUpdateFiles[0])
		afterFile, err := os.ReadFile(afterUpdateFiles[0])

		branchHr := v2beta1.HelmRelease{}
		err = yaml.Unmarshal(beforeFile, &branchHr)
		assert.Nil(t, err)

		testHr := v2beta1.HelmRelease{}
		err = yaml.Unmarshal(afterFile, &testHr)
		assert.Nil(t, err)

		// Get the diff between the HRs
		changes, err := diff.Diff(branchHr, testHr)
		for _, change := range changes {
			// Validate that each change is an "update"
			assert.Equal(t, diff.UPDATE, change.Type)
			// Validate that .spec.chart.spec.version is the only field that changes
			assert.Equal(t, []string{"Spec", "Chart", "Spec", "Version"}, change.Path)
			// Validate that the updated version is what we expect
			assert.Equal(t, updateToVersion, change.To)
		}
	}
}

func TestUpdateChartVersionsPathChanged(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := fmt.Sprintf(kommanderChartVersionTemplate, "v1.0.0")
	matches, err := filepath.Glob(filepath.Join(tmpDir, kommanderHelmReleasePathPattern))
	assert.Nil(t, err)
	assert.Equal(t, len(matches), 1)

	// change the Kommander HelmRelease filename
	os.Rename(matches[0], filepath.Join(filepath.Dir(matches[0]), "test.yaml"))

	err = updateChartVersions(tmpDir, updateToVersion)
	assert.Error(t, err, "expected chart version update to fail as the filename changed")
}

func TestUpdateChartVersionsVersionFormatChanged(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := fmt.Sprintf(kommanderChartVersionTemplate, "v1.0.0")
	matches, err := filepath.Glob(filepath.Join(tmpDir, kommanderHelmReleasePathPattern))
	assert.Nil(t, err)
	assert.Equal(t, len(matches), 1)

	// Change the chart version to something unexpected that would break the release automation
	parsedFile, err := envsubst.ParseFile(matches[0])
	subVars := map[string]string{
		"kommanderChartVersion": "foo",
		"releaseNamespace":      "${releaseNamespace}",
	}
	updatedFile, err := parsedFile.Execute(func(s string) string {
		return subVars[s]
	})

	err = os.WriteFile(matches[0], []byte(updatedFile), 0644)
	assert.Nil(t, err)

	err = updateChartVersions(tmpDir, updateToVersion)
	assert.Error(t, err, "expected chart version update to fail as the chart version was changed to something unexpected")
}
