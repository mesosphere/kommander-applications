package upgradematrix

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testFile = `upgrades:
  - from: v1.2.3
    to: v4.5.6
    k8s_version: "1.34"`

const expectedContent = `upgrades:
  - from:`

func TestUpdateUpgradeMatrix(t *testing.T) {
	dir := fetchRepo(t)

	// Replace the file with test data
	err := os.WriteFile(filepath.Join(dir, "upgrade-matrix.yaml"), []byte(testFile), 0644)
	require.NoError(t, err)

	err = UpdateUpgradeMatrix(context.Background(), dir)
	if errors.Is(err, ErrDKPNotFound) {
		t.Skip("Skipping test as gh dkp not installed.")
	}
	require.NoError(t, err)

	// Check that the file has been regenerated
	fileContent, err := os.ReadFile(filepath.Join(dir, "upgrade-matrix.yaml"))
	require.NoError(t, err)

	require.Contains(t, string(fileContent), expectedContent)
	require.NotContains(t, string(fileContent), testFile)
}

func fetchRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cmd := exec.Command("git", "clone", strings.Repeat("../", 4), dir)
	require.NoError(t, cmd.Run())

	return dir
}
