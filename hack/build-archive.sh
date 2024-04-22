#!/usr/bin/env bash

set -euox pipefail

ARCHIVE_NAME=$1

dir=$(mktemp --directory)

tar -xvf "${ARCHIVE_NAME}" --directory "$dir"
yq 'del(.resources[] | select(. == "ai-navigator-repos.yaml"))' --inplace "$dir"/common/helm-repositories/kustomization.yaml

# update the modified time
tar -cvzf "${ARCHIVE_NAME}" -C "$dir" .

# cleanup
rm -rf "$dir"
