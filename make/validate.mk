SKIP_APPLICATIONS ?= ai-navigator-rag,ai-navigator-app,ai-navigator-cluster-info-agent,nkp-pulse-management,nkp-pulse-workspace

.PHONY: list-images
list-images: $(NKP_CLI_BIN) $(YQ_BIN) #list-images-full
	echo "Removing applications from images.yaml: $(SKIP_APPLICATIONS)"
	@if [ -n "$(SKIP_APPLICATIONS)" ]; then \
		for app in $$(echo "$(SKIP_APPLICATIONS)" | tr ',' ' '); do \
			yq eval "del(.applications[] | select(.name == \"$$app\"))" -i $(REPO_ROOT)/images.yaml; \
		done; \
	fi
	echo "Images after removing applications:"
	cat $(REPO_ROOT)/images.yaml
	yq '.applications[].images[]' $(REPO_ROOT)/images.yaml | sort | uniq | grep -v "oci://" > images.txt
	echo "Final list of images in images.txt:"
	cat images.txt

.PHONY: list-images-full
list-images-full: $(NKP_CLI_BIN)
	$(NKP_CLI_BIN) validate catalog-repository --repo-dir $(REPO_ROOT) --config $(REPO_ROOT)/.bloodhound.yml --artifacts-output $(REPO_ROOT)/images.yaml
	echo "Generated images.yaml:"
	cat $(REPO_ROOT)/images.yaml
