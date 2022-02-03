.PHONY: flux-update
flux-update: ## Updates flux manifests
flux-update: install-tool.flux2 install-tool.kustomize install-tool.github-cli install-tool.yq
flux-update: ; $(info $(M) updating flux manifests)
	$(REPO_ROOT)/hack/flux/update-flux.sh
