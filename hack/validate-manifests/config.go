package main

type Config struct {
	SkipApplications map[string]bool
	ReplacementVars  map[string]string
	SkipTypes        map[string]bool
	AdditionalCRDs   []string

	EnableLegacyCertmanagerGroup bool
}

func DefaultConfig() Config {
	return Config{
		SkipApplications: map[string]bool{
			"kaptain": true,
		},
		ReplacementVars: map[string]string{
			"releaseNamespace":       "namespace",
			"workspaceNamespace":     "namespace",
			"certificatesIssuerName": "issuer",
		},
		SkipTypes: map[string]bool{
			"constraints.gatekeeper.sh/v1beta1": true,
		},
		AdditionalCRDs: []string{
			"https://github.com/jetstack/cert-manager/releases/download/v1.7.0/cert-manager.crds.yaml",
			"https://raw.githubusercontent.com/istio/istio/1.9.1/manifests/charts/base/crds/crd-all.gen.yaml",
		},
		EnableLegacyCertmanagerGroup: true,
	}
}
