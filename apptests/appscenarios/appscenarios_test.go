package appscenarios

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAbsolutePathTo(t *testing.T) {
	absAppPath, err := AbsolutePathTo("reloader")
	assert.NoError(t, err)

	expected := filepath.Join("kommander-applications", "services", "reloader")
	assert.Contains(t, absAppPath, expected)
	assert.NotEmpty(t, filepath.Base(absAppPath))
}
