DKP_BLOODHOUND_VERSION ?= 0.19.1
DKP_BLOODHOUND_BIN := $(LOCAL_DIR)/bin/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

SKIP_APPLICATIONS ?= ai-navigator-gateway,ai-navigator-app,ai-navigator-cluster-info-agent,nkp-pulse-management,nkp-pulse-workspace

$(DKP_BLOODHOUND_BIN):
	mkdir -p `dirname $@`
	curl -fsSL https://downloads.d2iq.com/dkp-bloodhound/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)_$(GOOS)_$(GOARCH).tar.gz | tar xz -O > $@
	chmod +x $@

.PHONY: validate-manifests
validate-manifests: $(DKP_BLOODHOUND_BIN)
	$(DKP_BLOODHOUND_BIN)

.PHONY: list-images
list-images: _SKIP_APPLICATIONS_FLAG := $(if $(SKIP_APPLICATIONS),--skip-applications $(SKIP_APPLICATIONS),)
list-images: $(DKP_BLOODHOUND_BIN)
	$(DKP_BLOODHOUND_BIN) --no-validation --list-artifacts --output-artifacts-file $(REPO_ROOT)/images.yaml $(_SKIP_APPLICATIONS_FLAG)
