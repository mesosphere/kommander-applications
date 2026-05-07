package appscenarios

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKnativeValidateUpgradeVersionStep(t *testing.T) {
	tests := []struct {
		name        string
		previous    string
		current     string
		expectedErr string
	}{
		{
			name:     "allows patch upgrade",
			previous: "1.20.0",
			current:  "1.20.1",
		},
		{
			name:     "allows one minor upgrade",
			previous: "1.20.1",
			current:  "1.21.0",
		},
		{
			name:        "rejects skipped minor upgrade",
			previous:    "1.19.5",
			current:     "1.21.0",
			expectedErr: "only supports one minor version upgrade at a time",
		},
		{
			name:        "rejects downgrade",
			previous:    "1.21.0",
			current:     "1.20.1",
			expectedErr: "downgrade is not allowed",
		},
		{
			name:        "rejects major upgrade",
			previous:    "1.21.0",
			current:     "2.0.0",
			expectedErr: "major version changes are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := knative{
				appPathPreviousVersion: filepath.Join("applications", "knative", tt.previous),
				appPathCurrentVersion:  filepath.Join("applications", "knative", tt.current),
			}

			err := k.ValidateUpgradeVersionStep()
			if tt.expectedErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.expectedErr)
		})
	}
}
