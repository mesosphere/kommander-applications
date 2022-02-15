CERT_MANAGER_VERSION ?= v1.7.1

.PHONY: update-crds
update-crds:
	curl -L https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml -o crds/cert-manager.crds.yaml
