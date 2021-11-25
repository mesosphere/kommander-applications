INCLUDE_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

include $(INCLUDE_DIR)make.mk
include $(INCLUDE_DIR)shell.mk
include $(INCLUDE_DIR)help.mk
include $(INCLUDE_DIR)repo.mk
include $(INCLUDE_DIR)ci.mk
include $(INCLUDE_DIR)docker.mk
include $(INCLUDE_DIR)flux.mk
include $(INCLUDE_DIR)tools.mk
include $(INCLUDE_DIR)release.mk
