package chartversion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/envsubst"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
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
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

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

		branchOciRepo := sourcev1b2.OCIRepository{}
		err = yaml.Unmarshal(beforeFile, &branchOciRepo)
		assert.Nil(t, err)

		testOciRepo := sourcev1b2.OCIRepository{}
		err = yaml.Unmarshal(afterFile, &testOciRepo)
		assert.Nil(t, err)

		// Get the diff between the HRs
		changes, err := diff.Diff(branchOciRepo, testOciRepo)
		assert.Nil(t, err)
		assert.NotEmpty(t, changes)

		for _, change := range changes {
			// Validate that each change is an "update"
			assert.Equal(t, diff.UPDATE, change.Type, "expected the chart version update to result in an update operation")
			// Validate that .spec.reference.tag is the only field that changes
			assert.Equal(t, []string{"Spec", "Reference", "Tag"}, change.Path, "expected .spec.ref.tag to be the only field that changed in the Kommander chart's OCIRepository")
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
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.Nil(t, err)

	// Read the kommander-operator cm.yaml directly (path moved into manifests)
	cmPath := filepath.Join(tmpDir, "common", "kommander-operator", "manifests", "cm.yaml")
	content, err := os.ReadFile(cmPath)
	require.NoError(t, err)

	assert.Equal(t,
		2,
		strings.Count(string(content), updateToVersion),
	)
}

func TestUpdateManagementOperatorsVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.Nil(t, err)

	operators := []string{
		"kommander-operator", "managementplane", "loggingstack", "nkpcluster", "upgradeplan",
	}

	for _, operator := range operators {
		content, err := os.ReadFile(filepath.Join(tmpDir, "common", operator, "flux-kustomization.yaml"))
		require.NoError(t, err)

		assert.Equal(t,
			1,
			strings.Count(string(content), updateToVersion),
		)
	}
}

func TestUpdateManagementOperatorManifestsVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.Nil(t, err)

	manifestPaths, err := filepath.Glob(filepath.Join(tmpDir, "common", "*", "manifests", "all.yaml"))
	assert.Nil(t, err)
	assert.NotEmpty(t, manifestPaths)

	for _, manifestPath := range manifestPaths {
		content, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), updateToVersion)
	}
}

func TestUpdateChartVersionsPathChanged(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

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
	assert.ErrorContains(t, err, "no matches found for HelmRelease path")
}

func TestUpdateChartVersionsVersionFormatChanged(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

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
	updatedFile, err := parsedFile.Execute(func(s string) (string, bool) {
		return subVars[s], true
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
	anotherDir, err := os.MkdirTemp(fmt.Sprintf("%s/applications/kommander/", tmpDir), "stuff")
	assert.Nil(t, err)
	helmReleaseDir := fmt.Sprintf("%s/helmrelease/", anotherDir)
	err = os.MkdirAll(helmReleaseDir, 0755)
	assert.Nil(t, err)
	f, err := os.Create(fmt.Sprintf("%s/kommander.yaml", helmReleaseDir))
	assert.Nil(t, err)
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	updateToVersion := "v1.0.0"
	err = UpdateChartVersions(tmpDir, updateToVersion)
	assert.ErrorContains(t, err, "found > 1 match for HelmRelease path")
}

// TestReplaceKommanderVersion_PreservesYamlCelRegexBackslashes guards against
// whole-file envsubst lexers that drop a backslash from "\\" in literal YAML (the
// github.com/drone/envsubst bug). github.com/fluxcd/pkg/envsubst only enables
// backslash escapes inside ${…}, so CEL "\\d" / "\\." outside substitutions stay intact.
func TestReplaceKommanderVersion_PreservesYamlCelRegexBackslashes(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "all.yaml")
	const (
		oldVer = "${kommanderChartVersion:=v2.0.0-dev}"
		newVer = "${kommanderChartVersion:=v9.9.9}"
	)
	// Raw string: \\ is two backslashes on disk (typical YAML for regex escapes in CEL).
	content := `rule: size(self) > 0 && self.matches('^v?(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(?:-[0-9A-Za-z-.]+)?(?:\\+[0-9A-Za-z-.]+)?$')
image: mesosphere/x:` + oldVer + "\n"
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))

	subVars := map[string]string{"kommanderChartVersion": newVer}
	require.NoError(t, replaceKommanderVersion(p, subVars))

	out, err := os.ReadFile(p)
	require.NoError(t, err)
	s := string(out)
	assert.Contains(t, s, newVer)
	// On-disk YAML must still have two backslashes before "d" (four in a Go "..." literal).
	assert.Contains(t, s, "[1-9]\\\\d*", "CEL pattern must keep \\\\d, not collapse to \\d")
}
