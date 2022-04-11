REPO_ROOT := $(CURDIR)

LOCAL_DIR := $(REPO_ROOT)/.local

GIT_COMMIT := $(shell git rev-parse "HEAD^{commit}")
export GIT_TAG ?= $(shell git describe --tags "$(GIT_COMMIT)^{commit}" --match v* --abbrev=0 2>/dev/null)
export GIT_CURRENT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

export GITHUB_ORG ?= mesosphere
export GITHUB_REPOSITORY ?= kommander-applications

ifneq ($(shell git status --porcelain 2>/dev/null; echo $$?), 0)
	export GIT_TREE_STATE := dirty
else
	export GIT_TREE_STATE :=
endif

.PHONY: repo.dev.tag
repo.dev.tag: ## Returns development tag
repo.dev.tag: install-tool.go.svu
ifeq ($(GIT_CURRENT_BRANCH),main)
	svu minor --pattern 'v[0-9].[0-9]{[0-9],}.[0-9]{[0-9],[0-9]-rc.*,-rc.*,}' --suffix dev --tag-mode=all-branches --no-metadata
else
	svu patch --pattern 'v[0-9].[0-9]{[0-9],}.[0-9]{[0-9],[0-9]-rc.*,-rc.*,}' --suffix dev --no-metadata
endif
