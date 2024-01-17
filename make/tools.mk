# Override this in your own top-level Makefile if this is in a different path in your repo.
GO_TOOLS_FILE ?= $(REPO_ROOT)/.go-tools

# Explicitly override GOBIN so it does not inherit from the environment - this allows for a truly
# self-contained build environment for the project.
override GOBIN := $(LOCAL_DIR)/bin
export GOBIN
export PATH := $(GOBIN):$(PATH)

ifneq ($(wildcard $(GO_TOOLS_FILE)),)
define install_go_tool
	mkdir -p $(GOBIN)
	CGO_ENABLED=0 go install -v $$(grep $1 $(GO_TOOLS_FILE))
endef

.PHONY: install-tool.go.%
install-tool.go.%: ## Installs specific go tool
install-tool.go.%: install-tool.golang ; $(info $(M) installing go tool $*)
	$(call install_go_tool,$*)
endif


ifneq ($(wildcard $(GO_TOOLS_FILE)),)
.PHONY: install-tools.go
install-tools.go: ## Install all go tools
install-tools.go: install-tool.golang ; $(info $(M) installing all go tools)
	cat $(GO_TOOLS_FILE) | xargs -L1 go install -v
endif

.PHONY: install-tools
install-tools: ## Install all tools
install-tools: ; $(info $(M) installing all tools)
	$(call install_tool,)

.PHONY: install-tool.%
install-tool.%: ## Install specific tool
install-tool.%: ; $(info $(M) installing $*)
	$(call install_tool,$*)

.PHONY: install-tool.gh-dkp
install-tool.gh-dkp: ; $(info $(M) installing $*)
	devbox run gh extensions install mesosphere/gh-dkp || gh dkp -h
