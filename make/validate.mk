DKP_BLOODHOUND_VERSION ?= 0.19.1
DKP_BLOODHOUND_BIN := $(LOCAL_DIR)/bin/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

SKIP_APPLICATIONS ?= ai-navigator-app,ai-navigator-cluster-info-agent,nkp-pulse-management,nkp-pulse-workspace

$(DKP_BLOODHOUND_BIN):
	mkdir -p `dirname $@`
	curl -fsSL https://downloads.d2iq.com/dkp-bloodhound/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)_$(GOOS)_$(GOARCH).tar.gz | tar xz -O > $@
	chmod +x $@

.PHONY: list-images
list-images: _SKIP_APPLICATIONS_FLAG := $(if $(SKIP_APPLICATIONS),--skip-applications $(SKIP_APPLICATIONS),)
list-images: $(DKP_BLOODHOUND_BIN)
	$(DKP_BLOODHOUND_BIN) --no-validation --list-artifacts --output-artifacts-file $(REPO_ROOT)/images.yaml $(_SKIP_APPLICATIONS_FLAG)

# TODO : update with stable version once available
NKP_CLI_VERSION := 2.16.0-dev.11
NKP_CLI := $(LOCAL_DIR)/bin/nkp_cli_v$(NKP_CLI_VERSION)
NKP_CLI_ASSET := nkp_v$(NKP_CLI_VERSION)_$(GOOS)_amd64
NKP_CLI_ARCHIVE := $(NKP_CLI_ASSET).tar.gz

$(NKP_CLI):
	mkdir -p $(dir $@)
	curl -LO "https://downloads.d2iq.com/dkp/v$(NKP_CLI_VERSION)/$(NKP_CLI_ARCHIVE)" && tar -xzf $(NKP_CLI_ARCHIVE) -C .
	mv ./nkp  $@

.PHONY: validate-manifests
validate-manifests: $(NKP_CLI)
	$(NKP_CLI) validate catalog-repository -v=3 --repo-dir=$(CURDIR)
