.PHONY: test
test:
	cd hack/validate-manifests && go run .
