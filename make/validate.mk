DKP_BLOODHOUND_VERSION ?= 0.10.1
DKP_BLOODHOUND_BIN := $(LOCAL_DIR)/bin/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)

$(DKP_BLOODHOUND_BIN):
	mkdir -p `dirname $@`
	curl -fsSL https://downloads.d2iq.com/dkp-bloodhound/dkp-bloodhound_v$(DKP_BLOODHOUND_VERSION)_linux_amd64.tar.gz | tar xz -O > $@
	chmod +x $@

.PHONY: validate-manifests
validate-manifests: $(DKP_BLOODHOUND_BIN)
	$(DKP_BLOODHOUND_BIN)
