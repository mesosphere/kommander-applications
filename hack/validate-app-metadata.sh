#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

set +e
MISSING_SERVICE_YAMLS="$(diff <(for x in applications/*/*/metadata.yaml; do dirname "$x"; done | sort -u) <(printf "%s\n" applications/*/* | grep -Ev '/README.md$') | grep -E '^>')"
readonly MISSING_SERVICE_YAMLS
set -e

if [ -n "${MISSING_SERVICE_YAMLS}" ]; then
  printf "The following applications have missing metadata.yaml files:\n\n%s\n" "${MISSING_SERVICE_YAMLS}"
  exit 1
fi
