package extraimages

import (
	"os"
	"path/filepath"
	"testing"

	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rootDir = "../../../../"

func TestUpdateExtraImages(t *testing.T) {
	tmpDir := t.TempDir()

	err := cp.Copy(rootDir, tmpDir)
	require.NoError(t, err)

	err = UpdateExtraImagesVersions(tmpDir, "v1.2.3")
	assert.NoError(t, err)

	afterUpgradeFile, err := filepath.Glob(filepath.Join(
		tmpDir, "services/kommander/*/extra-images.txt",
	))
	assert.NoError(t, err)

	assert.Len(t, afterUpgradeFile, 1)
	contentes, err := os.ReadFile(afterUpgradeFile[0])
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/mesosphere/kommander-applications-server:v1.2.3\n", string(contentes))
}
