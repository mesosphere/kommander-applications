package utils

import "github.com/drone/envsubst"

// SubstitionsFromMap provides a function for envsubst that will provide
// subsitution variables from given map.
func SubstitionsFromMap(m map[string]string) func(string) string {
	return func(key string) string {
		return m[key]
	}
}

// EnvsubstBytes is a helper that runs subtitions on bytes instead of a string.
func EnvsubstBytes(data []byte, mapper func(string) string) ([]byte, error) {
	result, err := envsubst.Eval(string(data), mapper)
	return []byte(result), err
}
