.PHONY: flux-update
flux-update: ## Updates flux manifests
flux-update:
flux-update: ; $(info $(M) updating flux manifests)
	$(REPO_ROOT)/hack/flux/update-flux.sh
