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

func TestReplaceContent(t *testing.T) {
	dir := fetchRepo(t)
	changes, err := ReplaceContent(context.Background(), dir, "0.99.99")
	require.NoError(t, err)
	assert.Equal(t, 4, changes)
}

func fetchRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cmd := exec.Command("git", "clone", strings.Repeat("../", 4), dir)
	require.NoError(t, cmd.Run())

	return dir
}

func TestRegexps(t *testing.T) {
	prefixIndex := varNames.SubexpIndex("prefix")
	suffixIndex := varNames.SubexpIndex("suffix")

	cases := []struct {
		input  string
		prefix string
		suffix string
	}{
		{
			input:  "kommander-0.4.0-d2iq-defaults",
			prefix: "kommander-",
			suffix: "-d2iq-defaults",
		},
		{
			input:  "kommander-0.4.0-overrides",
			prefix: "kommander-",
			suffix: "-overrides",
		},
		{
			input:  "kommander-appmanagement-0.4.0-d2iq-defaults",
			prefix: "kommander-appmanagement-",
			suffix: "-d2iq-defaults",
		},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			result := varNames.FindStringSubmatch(c.input)
			require.GreaterOrEqual(t, len(result), suffixIndex)
			assert.Equal(t, c.prefix, result[prefixIndex])
			assert.Equal(t, c.suffix, result[suffixIndex])

			reconstructed := result[prefixIndex] + "0.4.0" + result[suffixIndex]
			assert.Equal(t, c.input, reconstructed)
		})
	}
}
