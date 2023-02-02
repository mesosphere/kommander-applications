S3_BUCKET ?= "downloads.d2iq.com"
S3_PATH ?= "dkp/$(GIT_TAG)"
S3_ACL ?= "bucket-owner-full-control"

.PHONY: release
release: ARCHIVE_NAME = kommander-applications-$(GIT_TAG).tar.gz
release: install-tool.awscli
	git archive --format "tar.gz" -o $(ARCHIVE_NAME) \
	                              $(GIT_TAG) -- \
	                              common services
	aws s3 cp --acl $(S3_ACL) $(ARCHIVE_NAME) s3://$(S3_BUCKET)/$(S3_PATH)/
