DKP_BLOODHOUND_VERSION ?= 0.19.1
DKP_BLOODHOUND_BIN := $(LOCAL_DIR)/bin/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

SKIP_APPLICATIONS ?= ai-navigator-gateway,ai-navigator-app,ai-navigator-cluster-info-agent,nkp-pulse-management,nkp-pulse-workspace

$(DKP_BLOODHOUND_BIN):
	mkdir -p `dirname $@`
	curl -fsSL https://downloads.d2iq.com/dkp-bloodhound/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)_$(GOOS)_$(GOARCH).tar.gz | tar xz -O > $@
	chmod +x $@

.PHONY: list-images
list-images: _SKIP_APPLICATIONS_FLAG := $(if $(SKIP_APPLICATIONS),--skip-applications $(SKIP_APPLICATIONS),)
list-images: $(DKP_BLOODHOUND_BIN)
	$(DKP_BLOODHOUND_BIN) --no-validation --list-artifacts --output-artifacts-file $(REPO_ROOT)/images.yaml $(_SKIP_APPLICATIONS_FLAG)

NKP_CATALOG_CLI_VERSION ?= 0.2.0
NKP_CATALOG_CLI := $(LOCAL_DIR)/bin/nkp_catalog_cli_v$(NKP_CATALOG_CLI_VERSION)
TAG := v$(NKP_CATALOG_CLI_VERSION)
OWNER := nutanix-cloud-native
REPO := nkp-catalog-cli
ASSET := catalog_v$(NKP_CATALOG_CLI_VERSION)_$(GOOS)_$(GOARCH).tar.gz

$(NKP_CATALOG_CLI):
	mkdir -p $(dir $@)
	gh release download $(TAG) --repo $(OWNER)/$(REPO) --pattern $(ASSET) && tar -xzf ./$(ASSET) -C .
	mv ./nkp-catalog-cli $@ && chmod +x $@
	rm -f $(ASSET)

.PHONY: validate-manifests
validate-manifests: $(NKP_CATALOG_CLI)
	$(NKP_CATALOG_CLI) validate catalog-repository -v=3 --repo-dir=$(CURDIR)
