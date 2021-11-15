include make/all.mk

# Versions for tools that are not managed by asdf.
ASDF_VERSION=0.8.1

# Inputs required to build the CI Docker image with a determinstic tag.
CI_DOCKER_BUILD_ARGS=ASDF_VERSION=$(ASDF_VERSION)
CI_DOCKER_EXTRA_FILES=.tool-versions
