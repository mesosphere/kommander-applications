S3_BUCKET ?= "downloads.mesosphere.io"
S3_PATH ?= "dkp"
S3_ACL ?= "bucket-owner-full-control"

.PHONY: release
release: ARCHIVE_NAME = kommander-applications_$(GIT_TAG).tar.gz
release: install-tool.awscli
	git archive --format "tar.gz" -o $(ARCHIVE_NAME) \
	                              --prefix kommander-applications/ \
	                              $(GIT_TAG)
	aws s3 cp --acl $(S3_ACL) $(ARCHIVE_NAME) s3://$(S3_BUCKET)/$(S3_PATH)/
