.PHONY: dockerauth
dockerauth:
ifdef DOCKER_USERNAME
ifdef DOCKER_PASSWORD
	$(info $(M) Logging in to Docker Hub)
	echo -n $(DOCKER_PASSWORD) | devbox run docker login -u $(DOCKER_USERNAME) --password-stdin
endif
endif
