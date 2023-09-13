package updatecapimate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mesosphere/kommander-applications/hack/release/pkg/constants"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
)

const rootDir = "../../../../"

func TestUpdateCAPIMateVersionsSuccessfully(t *testing.T) {
	tmpDir := t.TempDir()

	// Make a copy of the current repo state to modify
	err := cp.Copy(rootDir, tmpDir)
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

	var n yaml.Node

	err = yaml.Unmarshal(beforeFile, &n)
	require.NoError(t, err)

	dataPath, err := yamlpath.NewPath(`$.data`)
	require.NoError(t, err)

	dataNode, err := dataPath.Find(&n)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(dataNode), 1)
	require.GreaterOrEqual(t, len(dataNode[0].Content), 2)

	// fmt.Printf("%+v\n", dataNode[0].Content[1])
	capiPath, err := yamlpath.NewPath(`$.capimate.image.tag`)
	require.NoError(t, err)

	err = yaml.Unmarshal([]byte(dataNode[0].Content[1].Value), &n)
	require.NoError(t, err)

	capiNode, err := capiPath.Find(&n)
	require.NoError(t, err)
	require.NotEmpty(t, capiNode)

	currentCapiVersion := capiNode[0].Value

	assert.NotContains(t, string(beforeFile), "tag: v1.0.0")
	assert.Contains(t, string(afterFile), "tag: v1.0.0")
	assert.NotContains(t, string(afterFile), "tag: "+currentCapiVersion)
}

func TestUpdateCAPIMateVersionsFailsWhenItCannotFindCM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prerelease")
	assert.Nil(t, err)
	defer os.RemoveAll(tmpDir)
	updateToVersion := "v1.0.0"
	err = UpdateCAPIMateVersion(tmpDir, updateToVersion)
	assert.ErrorContains(t, err, "verify the kommander-applications repo path")
}
