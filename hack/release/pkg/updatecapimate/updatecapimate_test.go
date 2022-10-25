package updatecapimate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
)

const rootDir = "../../../../"

func TestUpdateCAPIMateVersionsSuccessfully(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	// Make a copy of the current repo state to modify
	err = cp.Copy(rootDir, tmpDir)
	assert.Nil(t, err)

	updateToVersion := "v1.0.0"
	err = UpdateCAPIMateVersion(tmpDir, updateToVersion)
	assert.Nil(t, err)

	// Path to the file we want to update
	pathToChart := filepath.Join(constants.KommanderAppPath, "*/*/cm.yaml")

	beforeUpdateFiles, err := filepath.Glob(filepath.Join(rootDir, pathToChart))
	assert.Nil(t, err)

	// Get the tmp file that holds CAPIMate info
	afterUpdateFiles, err := filepath.Glob(filepath.Join(tmpDir, pathToChart))
	assert.Nil(t, err)

	beforeFile, err := os.ReadFile(beforeUpdateFiles[0])
	assert.Nil(t, err)

	afterFile, err := os.ReadFile(afterUpdateFiles[0])
	assert.Nil(t, err)

	assert.Contains(t, string(beforeFile), "tag: v0.0.0-dev.0")
	assert.NotContains(t, string(beforeFile), "tag: v1.0.0")
	assert.Contains(t, string(afterFile), "tag: v1.0.0")
	assert.NotContains(t, string(afterFile), "tag: v0.0.0-dev.0")

}

func TestUpdateCAPIMateVersionsFailsWhenItCannotFindCM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)
	updateToVersion := "v1.0.0"
	err = UpdateCAPIMateVersion(tmpDir, updateToVersion)
	assert.ErrorContains(t, err, "verify the kommander-applications repo path")
}
