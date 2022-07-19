package utils

import (
	"errors"
	"path/filepath"
	"runtime"
)

var errNotFoundSetupFile = errors.New("couldn't get path of the path_utils.go")

// GetRootDir returns absolute path to the root directory of the project.
func GetRootDir() (string, error) {
	// Get the absolute path of the current file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errNotFoundSetupFile
	}

	// Get the absolute path of the project root
	return filepath.Join(filepath.Dir(filename), "../../../"), nil
}
