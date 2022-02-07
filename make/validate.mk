.PHONY: validate-manifests
validate-manifests:
	cd hack/validate-manifests && go run .
