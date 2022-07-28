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
	dir := t.TempDir()
	cmd := exec.Command("git", "clone", strings.Repeat("../", 4), dir)
	require.NoError(t, cmd.Run())

	newVersion := "0.99.99"
	err := SetKommanderAppsVersion(context.Background(), dir, newVersion)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, constants.KommanderAppPath, newVersion))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, constants.KommanderAppMgmtPath, newVersion))
	assert.NoError(t, err)
}
