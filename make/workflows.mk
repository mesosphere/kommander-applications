.PHONY: workflow-labeler-yaml-update
workflow-labeler-yaml-update: ## Updates .github/service-labeler.yaml for use with labeler GH action
workflow-labeler-yaml-update: ; $(info $(M) updating .github/service-labeler.yaml with latest services)
	ls -d services/* | awk 'NR > 1' | sed p | awk 'NR % 2 { print $$0 ":" } !(NR % 2) {print "  - " $$0 "/**";}' > .github/service-labeler.yaml
