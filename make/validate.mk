SKIP_APPLICATIONS ?= ai-navigator-rag,ai-navigator-app,ai-navigator-cluster-info-agent,nkp-pulse-management,nkp-pulse-workspace

.PHONY: list-images
list-images: $(NKP_CLI_BIN) $(YQ_BIN) list-images-full
	echo "Removing applications from images.yaml: $(SKIP_APPLICATIONS)"
	yq eval 'del(.applications[] | select(.name as $$name | "$(SKIP_APPLICATIONS)" | split(",") | contains([$$name])))' -i $(REPO_ROOT)/images.yaml
	yq '.applications[].images[]' images.yaml | sort | uniq | grep -v "oci://" > images.txt

.PHONY: list-images-full
list-images-full: $(NKP_CLI_BIN)
	$(NKP_CLI_BIN) validate catalog-repository --repo-dir $(REPO_ROOT) --config $(REPO_ROOT)/.bloodhound.yml --artifacts-output $(REPO_ROOT)/images.yaml
