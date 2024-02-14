S3_BUCKET ?= "downloads.mesosphere.io"
S3_PATH ?= "dkp/$(GIT_TAG)"
S3_ACL ?= "bucket-owner-full-control"

.PHONY: release
release: ARCHIVE_NAME = kommander-applications-$(GIT_TAG).tar.gz
release: PUBLISHED_URL = https://downloads.d2iq.com/dkp/$(GIT_TAG)/$(ARCHIVE_NAME)
release:
	# We don't want to have ai-navigator in airgapped bundle
	# and we don't want to have ai-navigator-cluster-info-agent in airgapped bundle
	# the connected customers download the k-apps from GitHub where it is still present
	git archive --format "tar.gz" -o $(ARCHIVE_NAME) \
								  $(GIT_TAG) -- \
								  common services charts ":(exclude)services/ai-navigator-app" \
								  common services charts ":(exclude)services/ai-navigator-cluster-info-agent"
	aws s3 cp --acl $(S3_ACL) $(ARCHIVE_NAME) s3://$(S3_BUCKET)/$(S3_PATH)/
	echo "Published to $(PUBLISHED_URL)"
ifeq (,$(findstring dev,$(GIT_TAG)))
	# Make sure to set SLACK_WEBHOOK environment variable to webhook url for the below mentioned channel
	curl -X POST -H 'Content-type: application/json' \
	--data '{"channel":"#eng-shipit","blocks":[{"type":"header","text":{"type":"plain_text","text":":github: Kommander Applications Git Repo Tarball $(GIT_TAG) is out!","emoji":true}},{"type":"section","text":{"type":"mrkdwn","text":"$(PUBLISHED_URL)"}}]}' \
	$(SLACK_WEBHOOK)
endif
