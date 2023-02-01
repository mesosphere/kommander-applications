S3_BUCKET ?= "downloads.d2iq.com"
S3_PATH ?= "dkp/$(GIT_TAG)"
S3_ACL ?= "bucket-owner-full-control"

.PHONY: release
release: ARCHIVE_NAME = kommander-applications-$(GIT_TAG).tar.gz
release: PUBLISHED_URL = https://downloads.d2iq.com/dkp/$(GIT_TAG)/$(ARCHIVE_NAME)
release: install-tool.awscli
	git archive --format "tar.gz" -o $(ARCHIVE_NAME) \
	                              $(GIT_TAG) -- \
	                              common services
	aws s3 cp --acl $(S3_ACL) $(ARCHIVE_NAME) s3://$(S3_BUCKET)/$(S3_PATH)/
	echo "Published to $(PUBLISHED_URL)"
ifeq (,$(findstring dev,$(GIT_TAG)))
	# Make sure to set SLACK_WEBHOOK environment variable to webhook url for the below mentioned channel
	curl -X POST -H 'Content-type: application/json' \
	--data '{"channel":"#eng-shipit","blocks":[{"type":"header","text":{"type":"plain_text","text":":github: Kommander Applications Git Repo Tarball $(GIT_TAG) is out!","emoji":true}},{"type":"section","text":{"type":"mrkdwn","text":"$(PUBLISHED_URL)"}}]}' \
	$(SLACK_WEBHOOK)
endif
