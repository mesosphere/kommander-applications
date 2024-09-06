package upgradematrix

import (
	"context"
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
    k8s_version: "1.34"
`

const expectedContent = `upgrades:
  - from: v2.8.0
    to: v2.8.2-dev
    k8s_version: "1.28"
  - from: v2.8.1
    to: v2.8.2-dev
    k8s_version: "1.28"
  - from: v2.8.2-dev
    to: v2.12.0-dev
    k8s_version: "1.28"
`

func TestUpdateUpgradeMatrix(t *testing.T) {
	dir := fetchRepo(t)

	// Replace the existing file with known test data
	err := os.WriteFile(filepath.Join(dir, "upgrade-matrix.yaml"), []byte(testFile), 0644)
	require.NoError(t, err)

	// Expected update value
	err = os.Setenv(upgradeMatrixEnv, expectedContent)

	err = UpdateUpgradeMatrix(context.Background(), dir)
	require.NoError(t, err)

	// Check that the file has been regenerated
	fileContent, err := os.ReadFile(filepath.Join(dir, "upgrade-matrix.yaml"))
	require.NoError(t, err)

	require.Equal(t, string(fileContent), expectedContent)
	require.NotContains(t, string(fileContent), testFile)
}

func TestUpdateUpgradeMatrixNoEnv(t *testing.T) {
	dir := fetchRepo(t)

	// Replace the existing file with known test data
	err := os.WriteFile(filepath.Join(dir, "upgrade-matrix.yaml"), []byte(testFile), 0644)
	require.NoError(t, err)

	// Ensure that the environment variable is empty
	err = os.Setenv(upgradeMatrixEnv, "")

	err = UpdateUpgradeMatrix(context.Background(), dir)
	require.NoError(t, err)

	// Check that the file has *not* been regenerated
	fileContent, err := os.ReadFile(filepath.Join(dir, "upgrade-matrix.yaml"))
	require.NoError(t, err)

	require.Equal(t, string(fileContent), testFile)
}

func fetchRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cmd := exec.Command("git", "clone", strings.Repeat("../", 4), dir)
	require.NoError(t, cmd.Run())

	return dir
}
