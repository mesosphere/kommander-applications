INCLUDE_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(INCLUDE_DIR)ci.mk
include $(INCLUDE_DIR)repo.mk
