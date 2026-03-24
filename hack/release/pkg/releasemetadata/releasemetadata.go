package releasemetadata

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	stagingOCIURL = "oci://harbor.eng.nutanix.com/nkp_release_metadata/releases"

	releaseConfigDir                = "common/release/config"
	releaseOperatorVarsFile         = "release-operator-vars.yaml"
	configKustomizationFile         = "kustomization.yaml"
	fluxPreReleaseKustomizationFile = "common/release/flux-pre-release-kustomization.yaml"
	releaseKustomizationFile        = "common/release/kustomization.yaml"
	releaseFluxKustomizationFile    = "common/release/flux-kustomization.yaml"
)

const configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: release-operator-vars
data:
  releaseMetadataOCIURL: %s
  releaseMetadataOCITag: %s
`

const configKustomizationTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- release-operator-vars.yaml
namespace: ${releaseNamespace:-kommander}
`

const fluxPreReleaseKustomizationTemplate = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: release-operator-config
  namespace: "${releaseNamespace:-kommander}"
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: management
    namespace: kommander-flux
  path: ./common/release/config
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: kommander-vars
  prune: true
  wait: true
`

const releaseKustomizationTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- flux-pre-release-kustomization.yaml
- flux-kustomization.yaml
`

const fluxKustomizationWithDependsOnTemplate = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: release-operator
  namespace: "${releaseNamespace:-kommander}"
  annotations:
    managementplane.nkp.nutanix.com/version: "${kommanderChartVersion:=v2.18.0-dev}"
spec:
  dependsOn:
    - name: release-operator-config
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: management
    namespace: kommander-flux
  path: ./common/release/manifests
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: kommander-vars
  prune: true
  wait: true
`

// WriteReleaseOperatorConfig generates all the config files needed for pre-release
// staging OCI configuration. This includes:
// - common/release/config/release-operator-vars.yaml (ConfigMap)
// - common/release/config/kustomization.yaml
// - common/release/flux-pre-release-kustomization.yaml (Flux Kustomization)
// - Updates common/release/kustomization.yaml to include flux-pre-release-kustomization.yaml
// - Updates common/release/flux-kustomization.yaml to add dependsOn
func WriteReleaseOperatorConfig(repo, version string) error {
	configDir := filepath.Join(repo, releaseConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", configDir, err)
	}

	files := []struct {
		path    string
		content string
	}{
		{
			path:    filepath.Join(configDir, releaseOperatorVarsFile),
			content: fmt.Sprintf(configMapTemplate, stagingOCIURL, version),
		},
		{
			path:    filepath.Join(configDir, configKustomizationFile),
			content: configKustomizationTemplate,
		},
		{
			path:    filepath.Join(repo, fluxPreReleaseKustomizationFile),
			content: fluxPreReleaseKustomizationTemplate,
		},
		{
			path:    filepath.Join(repo, releaseKustomizationFile),
			content: releaseKustomizationTemplate,
		},
		{
			path:    filepath.Join(repo, releaseFluxKustomizationFile),
			content: fluxKustomizationWithDependsOnTemplate,
		},
	}

	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.path, err)
		}
	}

	return nil
}
