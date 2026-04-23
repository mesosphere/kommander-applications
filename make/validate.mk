SKIP_APPLICATIONS ?= ai-navigator-app,nkp-mcp-server,nkp-pulse-management,nkp-pulse-workspace

FULL_BUNDLE_FILE=artifacts_full.yaml
AIRGAPPED_BUNDLE_FILE=artifacts_airgapped.yaml
AIRGAPPED_BUNDLE_IMAGES_TXT=artifacts_airgapped_images.txt

.PHONY: list-airgapped-artifacts-yaml
list-airgapped-artifacts-yaml: $(NKP_CLI_BIN) $(YQ_BIN)
	cp $(REPO_ROOT)/$(FULL_BUNDLE_FILE) $(REPO_ROOT)/$(AIRGAPPED_BUNDLE_FILE)
	@for app in $$(echo "$(SKIP_APPLICATIONS)" | tr ',' ' '); do \
		yq eval "del(.applications[] | select(.name == \"$$app\"))" -i $(REPO_ROOT)/$(AIRGAPPED_BUNDLE_FILE); \
	done
	@echo "Images after removing applications ($(AIRGAPPED_BUNDLE_FILE)):" && cat $(REPO_ROOT)/$(AIRGAPPED_BUNDLE_FILE)
	yq '.applications[].images[]' $(REPO_ROOT)/$(AIRGAPPED_BUNDLE_FILE) | sort | uniq | grep -v "oci://" > $(REPO_ROOT)/$(AIRGAPPED_BUNDLE_IMAGES_TXT)
	@echo "Generated $(AIRGAPPED_BUNDLE_IMAGES_TXT):"
	@cat $(REPO_ROOT)/$(AIRGAPPED_BUNDLE_IMAGES_TXT)

.PHONY: generate-artifacts-yaml
generate-artifacts-yaml: $(NKP_CLI_BIN)
	$(NKP_CLI_BIN) validate catalog-repository --repo-dir $(REPO_ROOT) --config $(REPO_ROOT)/.bloodhound.yml --artifacts-output $(REPO_ROOT)/$(FULL_BUNDLE_FILE)
	@echo "Generated $(FULL_BUNDLE_FILE):"
	@cat $(REPO_ROOT)/$(FULL_BUNDLE_FILE)

.PHONY: validate-artifacts-yaml-in-sync
validate-artifacts-yaml-in-sync: generate-artifacts-yaml
	git diff --exit-code HEAD -- $(REPO_ROOT)/$(FULL_BUNDLE_FILE) || (printf "Error: $(FULL_BUNDLE_FILE) is out of date. Run 'make generate-artifacts-yaml' and commit.\n\n" && exit 1);

# Requires crane and yq on PATH; log in to registries that need auth (Docker Hub, ghcr, nvcr, …).
.PHONY: validate-upstream-container-images
validate-upstream-container-images: ## Probe artifacts_full.yaml + licenses.d2iq.yaml (crane)
	$(REPO_ROOT)/hack/validate-upstream-container-images.sh

# Fresh Bloodhound bundle (temp file) + same registry probes as validate-upstream-container-images.
.PHONY: validate-catalog-images
validate-catalog-images: $(NKP_CLI_BIN)
	NKP_BIN=$(NKP_CLI_BIN) $(REPO_ROOT)/hack/validate-bloodhound-catalog-images.sh

.PHONY: validate-bloodhound-catalog-images
validate-bloodhound-catalog-images: validate-catalog-images ## alias for validate-catalog-images
