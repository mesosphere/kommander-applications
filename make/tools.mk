.PHONY: install-tool.gh-dkp
install-tool.gh-dkp: ; $(info $(M) installing $*)
	gh extensions install mesosphere/gh-dkp || gh dkp -h
