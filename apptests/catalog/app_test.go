package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadRequiredDependencies(t *testing.T) {
	t.Run("returns empty when metadata file is missing", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		deps, err := readRequiredDependencies(tmpDir)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(deps) != 0 {
			t.Fatalf("expected no dependencies, got: %v", deps)
		}
	})

	t.Run("parses requiredDependencies from metadata", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		metadata := `schema: catalog.nkp.nutanix.com/v1/application-metadata
requiredDependencies:
  - istio
  - cert-manager
`
		if err := os.WriteFile(filepath.Join(tmpDir, "metadata.yaml"), []byte(metadata), 0o600); err != nil {
			t.Fatalf("writing metadata: %v", err)
		}

		deps, err := readRequiredDependencies(tmpDir)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(deps) != 2 {
			t.Fatalf("expected 2 dependencies, got %d (%v)", len(deps), deps)
		}
		if deps[0] != "istio" || deps[1] != "cert-manager" {
			t.Fatalf("unexpected dependencies: %v", deps)
		}
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "metadata.yaml"), []byte("requiredDependencies: [istio"), 0o600); err != nil {
			t.Fatalf("writing metadata: %v", err)
		}

		if _, err := readRequiredDependencies(tmpDir); err == nil {
			t.Fatal("expected yaml parse error, got nil")
		}
	})
}

func TestCatalogSearchRoots(t *testing.T) {
	t.Run("uses configured catalog paths first", func(t *testing.T) {
		base := t.TempDir()
		repoA := filepath.Join(base, "repo-a")
		repoB := filepath.Join(base, "repo-b")
		if err := os.MkdirAll(filepath.Join(repoA, "applications"), 0o755); err != nil {
			t.Fatalf("mkdir repo-a applications: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repoB, "applications"), 0o755); err != nil {
			t.Fatalf("mkdir repo-b applications: %v", err)
		}

		t.Setenv("APPTESTS_CATALOG_PATHS", repoA+","+repoB)
		roots, err := catalogSearchRoots(repoA)
		if err != nil {
			t.Fatalf("catalogSearchRoots returned error: %v", err)
		}

		expectedPrefix := []string{repoA, repoB}
		if len(roots) < len(expectedPrefix) {
			t.Fatalf("expected at least %d roots, got %d: %v", len(expectedPrefix), len(roots), roots)
		}
		if !reflect.DeepEqual(roots[:2], expectedPrefix) {
			t.Fatalf("expected first roots %v, got %v", expectedPrefix, roots[:2])
		}
	})
}
