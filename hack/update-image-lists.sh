#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
pushd "${REPO_ROOT}" &>/dev/null

while IFS= read -r repofile; do
  envsubst <"${repofile}" | \
    gojq --yaml-input --raw-output 'select(.spec.url != null) | (.metadata.name | gsub("\\."; "-"))+" "+.spec.url' | \
    xargs -l1 --no-run-if-empty -- helm repo add --force-update
done < <(grep -R -m1 -l '^kind: HelmRepository')

readonly METADATA_FILENAME="metadata.yaml"
for dir in $(find . -type f -name "${METADATA_FILENAME}" | grep -o "\(.*\)/" | sort -u); do
  pushd "${dir}" &>/dev/null

  while IFS= read -r hr; do
    pushd "$(dirname "${hr}")" &>/dev/null
    envsubst -i "$(basename "${hr}")" | \
      gojq --yaml-input --raw-output 'select(.spec.chart.spec.sourceRef.name != null) |
                                      select(.spec.chart.spec.sourceRef.kind == "HelmRepository") |
                                      (.spec.chart.spec.sourceRef.name | gsub("\\."; "-"))+"/"+.spec.chart.spec.chart+" --chart-version="+.spec.chart.spec.version' | \
      xargs -l1 --no-run-if-empty -- helm list-images --unique -f <(gojq --yaml-input -r '.data.["values.yaml"]' defaults/cm.yaml 2>/dev/null || true)
    popd &>/dev/null
  done < <(grep -R -m1 -l '^kind: HelmRelease')
  popd &>/dev/null
done
