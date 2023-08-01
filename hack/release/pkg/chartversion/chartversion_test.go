package chartversion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drone/envsubst"
	"github.com/fluxcd/helm-controller/api/v2beta1"
	cp "github.com/otiai10/copy"
	"github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
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
		assert.Nil(t, err)

		afterFile, err := os.ReadFile(afterUpdateFiles[0])
		assert.Nil(t, err)

		branchHr := v2beta1.HelmRelease{}
		err = yaml.Unmarshal(beforeFile, &branchHr)
		assert.Nil(t, err)

		testHr := v2beta1.HelmRelease{}
		err = yaml.Unmarshal(afterFile, &testHr)
		assert.Nil(t, err)

		// Get the diff between the HRs
		changes, err := diff.Diff(branchHr, testHr)
		assert.Nil(t, err)
		assert.NotEmpty(t, changes)

		for _, change := range changes {
			// Validate that each change is an "update"
			assert.Equal(t, diff.UPDATE, change.Type, "expected the chart version update to result in an update operation")
			// Validate that .spec.chart.spec.version is the only field that changes
			assert.Equal(t, []string{"Spec", "Chart", "Spec", "Version"}, change.Path, "expected .spec.chart.spec.version to be the only field that changed in the Kommander HelmRelease")
			// Validate that the updated version is what we expect
			assert.Equal(t,
				fmt.Sprintf(kommanderChartVersionTemplate, updateToVersion),
				change.To,
				"expected the chart version to be updated to %s, but got %s", updateToVersion, change.To)
		}
	}
}

func TestUpdateKommanderOperatorVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.Nil(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, kommanderOperatorDefaultsCMPath))
	require.NoError(t, err)

	assert.Equal(t,
		2,
		strings.Count(string(content), updateToVersion),
	)
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
	err = os.Rename(matches[0], filepath.Join(filepath.Dir(matches[0]), "test.yaml"))
	assert.Nil(t, err)

	err = UpdateChartVersions(tmpDir, updateToVersion)
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
	assert.Nil(t, err)
	subVars := map[string]string{
		"kommanderChartVersion": "foo",
		"releaseNamespace":      "${releaseNamespace}",
	}
	updatedFile, err := parsedFile.Execute(func(s string) string {
		return subVars[s]
	})
	assert.Nil(t, err)

	err = os.WriteFile(matches[0], []byte(updatedFile), 0o644)
	assert.Nil(t, err)

	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.Error(t, err, "expected chart version update to fail as the chart version was changed to something unexpected")
}

func TestUpdateChartVersionsTooManyFiles(t *testing.T) {
	// Make a new temp dir to copy the repo state into
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)
	// Make a new temp dir to put a redundant file in
	anotherDir, err := os.MkdirTemp(fmt.Sprintf("%s/services/kommander/", tmpDir), "stuff")
	assert.Nil(t, err)
	f, err := os.Create(fmt.Sprintf("%s/kommander.yaml", anotherDir))
	assert.Nil(t, err)
	defer f.Close()
	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.ErrorContains(t, err, "found > 1 match for HelmRelease path")
}

func TestUpdatePreUpgradeImagesq(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.Nil(t, err)

	preUpgradePaths := []string{kubecostPreUpgradePath, gatekeeperPreUpgradePath, loggingOperatorPreUpgradePath}

	for _, path := range preUpgradePaths {
		t.Run(path, func(t *testing.T) {
			updatedFile, err := filepath.Glob(filepath.Join(tmpDir, path))
			assert.Nil(t, err)
			assert.Len(t, updatedFile, 1)

			content, err := os.ReadFile(updatedFile[0])
			require.NoError(t, err)

			assert.Equal(t,
				1,
				strings.Count(string(content), updateToVersion),
			)
		})
	}

}
