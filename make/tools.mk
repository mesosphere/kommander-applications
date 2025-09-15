NKP_CLI_VERSION ?= v0.0.0-dev.0
YQ_VERSION ?= v4.47.2

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

YQ_BIN := $(LOCAL_DIR)/bin/yq_$(YQ_VERSION)
NKP_CLI_BIN := $(LOCAL_DIR)/bin/nkp_v$(NKP_CLI_VERSION)

.PHONY: install-tool.gh-dkp
install-tool.gh-dkp: ; $(info $(M) installing $*)
	gh extensions install mesosphere/gh-dkp || gh dkp -h

$(YQ_BIN):
	mkdir -p $(dir $@)
	curl -fsSLo $@ https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(GOOS)_$(GOARCH)
	chmod +x $@

$(NKP_CLI_BIN):
	mkdir -p `dirname $@`
	curl -fsSL https://downloads.d2iq.com/dkp/$(NKP_CLI_VERSION)/nkp_$(NKP_CLI_VERSION)_$(GOOS)_amd64.tar.gz | tar xz -O > $@
	chmod +x $@
