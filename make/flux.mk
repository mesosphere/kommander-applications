.PHONY: flux-update
flux-update: ## Updates flux manifests
flux-update: ; $(info $(M) updating flux manifests)
	$(REPO_ROOT)/hack/flux/update-flux.sh

.PHONY: flux-oci-mirror-update
flux-oci-mirror-update: ## Updates flux oci mirror manifests
flux-oci-mirror-update: ; $(info $(M) updating flux oci mirorr manifests)
	$(REPO_ROOT)/hack/flux/update-flux-oci-mirror.sh
