package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogRootUsesNKPCatalogDir(t *testing.T) {
	catalogDir := t.TempDir()
	t.Setenv(catalogDirEnv, catalogDir)

	root, err := catalogRoot()
	require.NoError(t, err)
	assert.Equal(t, catalogDir, root)
}

func TestApplicationDirUsesCatalogRoot(t *testing.T) {
	catalogDir := t.TempDir()
	t.Setenv(catalogDirEnv, catalogDir)

	dir, err := applicationDir("reloader")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(catalogDir, "applications", "reloader"), dir)
}

func TestVersionDirsSkipsDotPrefixedAndFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "1.0.0"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "2.0.0"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644))

	got, err := versionDirs(dir)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(dir, "1.0.0"),
		filepath.Join(dir, "2.0.0"),
	}, got)
}

func TestAbsolutePathToHonorsVersionAndLatest(t *testing.T) {
	catalogDir := t.TempDir()
	appDir := filepath.Join(catalogDir, "applications", "kueue")
	require.NoError(t, os.MkdirAll(filepath.Join(appDir, "0.17.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(appDir, "0.18.0"), 0o755))
	t.Setenv(catalogDirEnv, catalogDir)

	specific, err := absolutePathTo("kueue", "0.17.0")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(appDir, "0.17.0"), specific)

	latest, err := absolutePathTo("kueue", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(appDir, "0.18.0"), latest)
}

func TestHasPreviousVersion(t *testing.T) {
	catalogDir := t.TempDir()
	appDir := filepath.Join(catalogDir, "applications", "kueue")
	require.NoError(t, os.MkdirAll(filepath.Join(appDir, "0.17.0"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(appDir, "0.18.0"), 0o755))
	t.Setenv(catalogDirEnv, catalogDir)

	assert.True(t, NewAppScenario("kueue", "0.18.0").(*App).HasPreviousVersion())
	assert.False(t, NewAppScenario("kueue", "0.17.0").(*App).HasPreviousVersion())
	assert.True(t, NewAppScenario("kueue", "").(*App).HasPreviousVersion())
}
