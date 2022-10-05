.PHONY: workflow-labeler-yaml-update
# ls -d services/* | awk 'NR > 1' | sed p
#   Output path to all services (e.g. "services/centralized-grafana"), printing each line twice
# awk 'NR % 2 { print $$0 ":" } !(NR % 2) {print "  - " $$0 "/**";}'
#   For every odd line, suffix with ":" (This is the label name to be applied)
#   For every even line, format it like "  - services/centralized-grafana/**" (This is the changed file path for which to apply the label)
# The output file is formatted as required by the labeler workflow.
workflow-labeler-yaml-update: ## Updates .github/service-labeler.yaml for use with labeler GH action
workflow-labeler-yaml-update: ; $(info $(M) updating .github/service-labeler.yaml with latest services)
	ls -d services/* | awk 'NR > 1' | sed p | awk 'NR % 2 { print $$0 ":" } !(NR % 2) {print "  - " $$0 "/**";}' > .github/service-labeler.yaml
