KOMMANDER_E2E_DIR  = $(REPO_ROOT)/.tmp/kommander-e2e

# E2E configurations
E2E_TIMEOUT       ?= 120m
# Using a custom build of KinD node docker image allows us to test against any patch version
# of Kubernetes. The image is built using exactly the same methodology as upstream, namely
# `kind build node-image`, but upstream only builds a few k8s patch versions, whereas we
# build image for every patch release of Kubernetes. This allows us to be slightly more
# flexible with our testing as well as testing against the same patch version as we deliver
# by default with NKP.
# See https://github.com/mesosphere/kind-docker-image-automation/ for the build repo.
E2E_KINDEST_IMAGE ?= "ghcr.io/mesosphere/kind-node-ci:v1.26.6"

# Kommander applications are upgraded on the previous k8s version first, before the cluster
# is upgraded. Therefore, for the upgrade test, we should use the previous k8s version to
# more accurately mimic an actual upgrade scenario.
E2E_KINDEST_IMAGE_FOR_UPGRADE_TEST ?= "ghcr.io/mesosphere/kind-node-ci:v1.26.6"
UPGRADE_FROM_VERSION ?= "v2.6.0-dev"

# (aweris): This should be a temporary workaround for v2.3.0 development. If you're still see clone test in v2.4.0
# it means "a temporary workaround" actually means "permanent solution".
.PHONY: kommander-e2e
kommander-e2e: ## Clones the kommander-e2e repo locally or updates the clone
kommander-e2e:
	@if [ -d $(KOMMANDER_E2E_DIR) ] ; then \
		cd $(KOMMANDER_E2E_DIR) && \
			git fetch origin && \
			git reset --hard origin/main ; \
	else \
		mkdir -p $(KOMMANDER_E2E_DIR) && \
			git clone -q https://github.com/mesosphere/kommander-e2e.git $(KOMMANDER_E2E_DIR) && \
			cd $(KOMMANDER_E2E_DIR) && \
			git checkout main ; \
	fi

.PHONY: test.e2e.install
test.e2e.install: kommander-e2e ; $(info $(M) running end-to-end kommander install test from kommander-e2e)
	cd $(KOMMANDER_E2E_DIR) && \
		E2E_TIMEOUT=$(E2E_TIMEOUT) \
		E2E_KINDEST_IMAGE=$(E2E_KINDEST_IMAGE) \
		E2E_TEST_PATH="feature/install/suites/kindcluster" \
		E2E_KOMMANDER_APPLICATIONS_REPOSITORY="github.com/mesosphere/kommander-applications.git?ref=$(GIT_COMMIT)" \
		E2E_DKP_ESSENTIAL_LICENSE=$(E2E_DKP_ESSENTIAL_LICENSE) \
		E2E_DKP_ENTERPRISE_LICENSE=$(E2E_DKP_ENTERPRISE_LICENSE) \
		VERBOSE=$(VERBOSE) \
		make test.e2e

.PHONY: test.e2e.upgrade.singlecluster
test.e2e.upgrade.singlecluster: kommander-e2e ; $(info $(M) running end-to-end kommander upgrade $(UPGRADE_FROM_VERSION) to $(GIT_COMMIT) test from kommander-e2e)
	cd $(KOMMANDER_E2E_DIR) && \
		E2E_TEST_PATH="feature/upgrade/suites/kind/singlecluster" \
		E2E_TIMEOUT=$(E2E_TIMEOUT) \
		E2E_KINDEST_IMAGE=$(E2E_KINDEST_IMAGE_FOR_UPGRADE_TEST) \
		E2E_KOMMANDER_APPLICATIONS_REPOSITORY="github.com/mesosphere/kommander-applications.git?ref=$(UPGRADE_FROM_VERSION)" \
		E2E_KOMMANDER_APPLICATIONS_REPOSITORY_TO_UPGRADE="github.com/mesosphere/kommander-applications.git?ref=$(GIT_COMMIT)" \
		VERBOSE=$(VERBOSE) \
		make test.e2e
