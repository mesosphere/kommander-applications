package releasemetadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReleaseOperatorConfig(t *testing.T) {
	tmpDir := t.TempDir()

	setupBaseFiles(t, tmpDir)

	version := "v2.18.0-dev.12"
	err := WriteReleaseOperatorConfig(tmpDir, version)
	require.NoError(t, err)

	t.Run("creates config directory", func(t *testing.T) {
		configDir := filepath.Join(tmpDir, releaseConfigDir)
		info, err := os.Stat(configDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("creates release-operator-vars ConfigMap", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, releaseConfigDir, releaseOperatorVarsFile))
		require.NoError(t, err)

		assert.Contains(t, string(content), "name: release-operator-vars")
		assert.Contains(t, string(content), "releaseMetadataOCIURL: "+stagingOCIURL)
		assert.Contains(t, string(content), "releaseMetadataOCITag: "+version)
	})

	t.Run("creates config kustomization.yaml", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, releaseConfigDir, configKustomizationFile))
		require.NoError(t, err)

		assert.Contains(t, string(content), "kind: Kustomization")
		assert.Contains(t, string(content), "- release-operator-vars.yaml")
		assert.Contains(t, string(content), "namespace: ${releaseNamespace:-kommander}")
	})

	t.Run("creates flux-pre-release-kustomization.yaml", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, fluxPreReleaseKustomizationFile))
		require.NoError(t, err)

		assert.Contains(t, string(content), "name: release-operator-config")
		assert.Contains(t, string(content), "path: ./common/release/config")
		assert.Contains(t, string(content), "wait: true")
	})

	t.Run("updates release kustomization.yaml with flux-pre-release-kustomization", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, releaseKustomizationFile))
		require.NoError(t, err)

		assert.Contains(t, string(content), "- flux-pre-release-kustomization.yaml")
		assert.Contains(t, string(content), "- flux-kustomization.yaml")
	})

	t.Run("updates flux-kustomization.yaml with dependsOn", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, releaseFluxKustomizationFile))
		require.NoError(t, err)

		assert.Contains(t, string(content), "dependsOn:")
		assert.Contains(t, string(content), "- name: release-operator-config")
		assert.Contains(t, string(content), "name: release-operator")
	})
}

func TestWriteReleaseOperatorConfig_DifferentVersions(t *testing.T) {
	testCases := []struct {
		name    string
		version string
	}{
		{name: "dev version", version: "v2.18.0-dev.12"},
		{name: "rc version", version: "v2.18.0-rc.1"},
		{name: "release version", version: "v2.18.0"},
		{name: "patch version", version: "v2.18.1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			setupBaseFiles(t, tmpDir)

			err := WriteReleaseOperatorConfig(tmpDir, tc.version)
			require.NoError(t, err)

			content, err := os.ReadFile(filepath.Join(tmpDir, releaseConfigDir, releaseOperatorVarsFile))
			require.NoError(t, err)

			assert.Contains(t, string(content), "releaseMetadataOCITag: "+tc.version)
			assert.Contains(t, string(content), "releaseMetadataOCIURL: "+stagingOCIURL)
		})
	}
}

func TestWriteReleaseOperatorConfig_AllFilesCreated(t *testing.T) {
	tmpDir := t.TempDir()
	setupBaseFiles(t, tmpDir)

	err := WriteReleaseOperatorConfig(tmpDir, "v1.0.0")
	require.NoError(t, err)

	expectedFiles := []string{
		filepath.Join(tmpDir, releaseConfigDir, releaseOperatorVarsFile),
		filepath.Join(tmpDir, releaseConfigDir, configKustomizationFile),
		filepath.Join(tmpDir, fluxPreReleaseKustomizationFile),
		filepath.Join(tmpDir, releaseKustomizationFile),
		filepath.Join(tmpDir, releaseFluxKustomizationFile),
	}

	for _, f := range expectedFiles {
		_, err := os.Stat(f)
		assert.NoError(t, err, "expected file to exist: %s", f)
	}
}

func setupBaseFiles(t *testing.T, tmpDir string) {
	t.Helper()

	releaseDir := filepath.Join(tmpDir, "common", "release")
	err := os.MkdirAll(releaseDir, 0o755)
	require.NoError(t, err)

	manifestsDir := filepath.Join(releaseDir, "manifests")
	err = os.MkdirAll(manifestsDir, 0o755)
	require.NoError(t, err)

	kustomizationContent := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- flux-kustomization.yaml
`
	err = os.WriteFile(filepath.Join(releaseDir, "kustomization.yaml"), []byte(kustomizationContent), 0o644)
	require.NoError(t, err)

	fluxKustomizationContent := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: release-operator
  namespace: "${releaseNamespace:-kommander}"
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: management
    namespace: kommander-flux
  path: ./common/release/manifests
  prune: true
  wait: true
`
	err = os.WriteFile(filepath.Join(releaseDir, "flux-kustomization.yaml"), []byte(fluxKustomizationContent), 0o644)
	require.NoError(t, err)
}
