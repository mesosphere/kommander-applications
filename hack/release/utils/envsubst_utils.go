package utils

import (
	"github.com/drone/envsubst"
)

// EvalFile evaluates the given file based on the given variables and returns rendered file contents.
func EvalFile(filePath string, vars map[string]string) (string, error) {
	parsedFile, err := envsubst.ParseFile(filePath)
	if err != nil {
		return "", err
	}

	updatedFile, err := parsedFile.Execute(func(s string) string {
		return vars[s]
	})
	if err != nil {
		return "", err
	}

	return updatedFile, nil
}
