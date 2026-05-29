package appversion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMove(t *testing.T) {
	dir := fetchRepo(t)

	newVersion := "0.99.99"
	err := SetKommanderAppsVersion(context.Background(), dir, newVersion)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, constants.KommanderAppPath, newVersion))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, constants.KommanderAppMgmtPath, newVersion))
	assert.NoError(t, err)
}

func TestReplaceContentInFile_BloodhoundPathVersion(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".bloodhound.yml")
	content := `paths:
  - applications/kommander/0.18.0/dynamic-helmreleases
`
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))

	changes, err := replaceContentInFile(context.Background(), f, "0.99.99")
	require.NoError(t, err)
	assert.Equal(t, 1, changes)

	got, err := os.ReadFile(f)
	require.NoError(t, err)
	assert.Contains(t, string(got), "applications/kommander/0.99.99/dynamic-helmreleases")
	assert.NotContains(t, string(got), "applications/kommander/0.18.0/dynamic-helmreleases")
}

func fetchRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cmd := exec.Command("git", "clone", strings.Repeat("../", 4), dir)
	require.NoError(t, cmd.Run())

	return dir
}
