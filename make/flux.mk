.PHONY: flux-update
flux-update: ## Updates flux manifests
flux-update: ; $(info $(M) updating flux manifests)
	$(REPO_ROOT)/hack/flux/update-flux.sh

.PHONY: flux-oci-mirror-update
flux-oci-mirror-update: ## Updates flux oci mirror manifests
flux-oci-mirror-update: ; $(info $(M) updating flux oci mirorr manifests)
	$(REPO_ROOT)/hack/flux/update-flux-oci-mirror.sh

# ===== Targets equivalent to kommander/make/dev.mk (copy & clean) =====

# Branch or tag of kommander-applications repository. Used for CI testing as well as airgapped bundle generation.
KOMMANDER_APPLICATIONS_REF ?= main

# Go-getter URI for kommander-applications. Encode '/' in ref for go-getter stability.
KOMMANDER_APPLICATIONS_URI ?= git::https://github.com/mesosphere/kommander-applications?ref=$(shell printf '%s' $(KOMMANDER_APPLICATIONS_REF) | sed 's|/|%2F|g')

# Temporary working copy of kommander-applications
export KOMMANDER_APPLICATIONS_DIR := $(REPO_ROOT)/kommander-applications/$(shell mktemp -d 2>/dev/null || mktemp -d -t kommander-apps | xargs basename)

# yq binary is provided by make/tools.mk via $(YQ_BIN). Provide a convenience alias to install it.
.PHONY: install-tools
install-tools: ## Ensures yq is installed
install-tools: $(YQ_BIN)

.PHONY: go-getter
go-getter: ## Verifies go-getter is installed on PATH
	@command -v go-getter >/dev/null || { echo "ERROR: go-getter not found. Install e.g. 'brew install go-getter'"; exit 127; }

define print-target
	@echo "\n>>> $@\n"
endef

$(KOMMANDER_APPLICATIONS_DIR): ## Prepares temporary kommander-applications workspace directory
	$(call print-target)
	rm -rf $(REPO_ROOT)/kommander-applications && mkdir -p $(KOMMANDER_APPLICATIONS_DIR)

.PHONY: kommander-applications
kommander-applications: ## Fetches kommander-applications at KOMMANDER_APPLICATIONS_REF into a temp directory
kommander-applications: install-tools go-getter
kommander-applications: TMP_DIR := $(shell mktemp -d 2>/dev/null || mktemp -d -t kommander-apps)
kommander-applications: $(KOMMANDER_APPLICATIONS_DIR)
	$(call print-target)
	@echo "Fetching: $(KOMMANDER_APPLICATIONS_URI) -> $(TMP_DIR)"
	go-getter $(KOMMANDER_APPLICATIONS_URI) $(TMP_DIR)

	rsync -a --no-links "$(TMP_DIR)/" "$(KOMMANDER_APPLICATIONS_DIR)/"
	rm -rf "$(TMP_DIR)"

.PHONY: copy-flux-manifests
copy-flux-manifests: ## Copies flux manifests from kommander-applications into pkg/embedded/flux-manifests
copy-flux-manifests: kommander-applications $(YQ_BIN)
	$(call print-target)

	@if command -v gfind >/dev/null 2>&1; then FIND=gfind; else FIND=find; fi; \
	if command -v gsort >/dev/null 2>&1; then SORT=gsort; else SORT=sort; fi; \
	if [ -z "$$FLUX_VERSION" ]; then \
		export FLUX_VERSION="$$($$FIND '$(KOMMANDER_APPLICATIONS_DIR)/applications/kommander-flux' -maxdepth 1 -type d | sed -nE 's|.*/([0-9]+\.[0-9]+\.[0-9]+)$$|\1|p' | $$SORT -V | tail -n1)"; \
	fi; \
	[ -n "$$FLUX_VERSION" ] || { echo 'ERROR: No x.y.z version directories found under $(KOMMANDER_APPLICATIONS_DIR)/applications/kommander-flux'; exit 1; }; \
	echo "Using Flux version $$FLUX_VERSION"; \
	SRC_DIR="$(KOMMANDER_APPLICATIONS_DIR)/applications/kommander-flux/$$FLUX_VERSION"; \
	DST_DIR="$(REPO_ROOT)/pkg/embedded/flux-manifests"; \
	mkdir -p "$$DST_DIR"; \
	# Copy flux manifests (kustomization, templates, mirror, and patches) into embed directory
	rsync -a --delete --exclude '.git' "$$SRC_DIR/kustomization.yaml" "$$DST_DIR/"; \
	if [ -d "$$SRC_DIR/templates" ]; then rsync -a --delete "$$SRC_DIR/templates/" "$$DST_DIR/templates/"; fi; \
	if [ -d "$$SRC_DIR/mirror" ]; then rsync -a --delete "$$SRC_DIR/mirror/" "$$DST_DIR/mirror/"; fi; \
	# include known patch files if present
	for f in patch-proxy-env-vars.yaml patch-source-ctrl-network-policy.yaml; do \
		if [ -f "$$SRC_DIR/$$f" ]; then rsync -a "$$SRC_DIR/$$f" "$$DST_DIR/"; fi; \
	 done; \
	# ensure yq is available for potential follow-up processing by consumers
	[ -x "$(YQ_BIN)" ] || { echo "ERROR: yq not installed at $(YQ_BIN)"; exit 1; }

ifneq ($(CLEAN_AFTER_COPY),)
	@echo "Cleaning up $(REPO_ROOT)/kommander-applications"
	rm -rf "$(REPO_ROOT)/kommander-applications"
endif

.PHONY: clean-kommander-applications
clean-kommander-applications: ## Removes temporary kommander-applications workspace
	$(call print-target)
	rm -rf "$(REPO_ROOT)/kommander-applications"
	@echo 'Removed $(REPO_ROOT)/kommander-applications'

.PHONY: clean-flux-manifests
clean-flux-manifests: ## Removes generated pkg/embedded/flux-manifests directory
	$(call print-target)
	rm -rf "$(REPO_ROOT)/pkg/embedded/flux-manifests/"
	@echo "Cleaned pkg/embedded/flux-manifests/"
