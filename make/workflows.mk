.PHONY: workflow-labeler-yaml-update
# ls -d applications/* | awk 'NR > 1'
#   Output path to all applications (e.g. "applications/centralized-grafana"), printing each line twice.
# awk '{ print $$0 ":\n- changed-files:\n  - any-glob-to-any-file:\n    - " $$0 "/**" }'
#   For every line (app), apply the required configuraton file structure for each match object.
# Each app points to the changed path glob for which to apply the label.
# The output file is formatted as required by the labeler workflow (https://github.com/actions/labeler).
workflow-labeler-yaml-update: ## Updates .github/app-labeler.yaml for use with labeler GH action
workflow-labeler-yaml-update: ; $(info $(M) updating .github/app-labeler.yaml with latest applications)
	ls -d applications/* | awk 'NR > 1' | awk '{ print $$0 ":\n- changed-files:\n  - any-glob-to-any-file:\n    - " $$0 "/**" }' > .github/app-labeler.yaml
