#!/usr/bin/env bash
# Check image refs from artifacts_full.yaml and licenses.d2iq.yaml with crane digest.
#
# On failure (exit 1), prints only missing refs to stdout, one per line. Otherwise quiet.
# Requires: yq, crane. Log in to registries separately (docker login / crane auth).
#
# Env:
#   ARTIFACTS_FILE  default: repo root artifacts_full.yaml
#   LICENSE_FILE    default: licenses.d2iq.yaml
#   KOMMANDER       tag for ${kommander} in licenses (default: from artifacts)
#   SKIP_LICENSES=1 only artifacts
#   SKIP_ARTIFACTS=1 only licenses
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACTS_FILE="${ARTIFACTS_FILE:-${ROOT}/artifacts_full.yaml}"
LICENSE_FILE="${LICENSE_FILE:-${ROOT}/licenses.d2iq.yaml}"
SKIP_LICENSES="${SKIP_LICENSES:-0}"
SKIP_ARTIFACTS="${SKIP_ARTIFACTS:-0}"

command -v yq >/dev/null 2>&1 || {
  echo "yq is required on PATH" >&2
  exit 1
}
command -v crane >/dev/null 2>&1 || {
  echo "crane is required on PATH" >&2
  exit 1
}

KOMMANDER="${KOMMANDER:-}"
if [[ -z "$KOMMANDER" && -f "$ARTIFACTS_FILE" ]]; then
  KOMMANDER="$(grep -m1 'docker.io/mesosphere/kommander2-core-installer:' "$ARTIFACTS_FILE" | sed 's/.*://' || true)"
fi

refs=()

add_ref() {
  local raw="$1" norm
  [[ -n "$raw" ]] || return 0
  [[ "$raw" == *'<registry-url>'* ]] && return 0
  if [[ "$raw" == *"\${"* ]] || [[ "$raw" == *"\`"* ]]; then
    return 0
  fi
  norm="${raw#oci://}"
  [[ -n "$norm" ]] || return 0
  if [[ "$norm" == */*:* ]]; then
    local first="${norm%%/*}"
    if [[ "$first" != *.* && "$first" != localhost && "$first" != localhost:* ]]; then
      norm="docker.io/${norm}"
    fi
  fi
  refs+=("$norm")
}

if [[ "$SKIP_ARTIFACTS" != "1" ]]; then
  [[ -f "$ARTIFACTS_FILE" ]] || {
    echo "Artifacts file not found: $ARTIFACTS_FILE" >&2
    exit 1
  }
  while IFS= read -r img; do
    add_ref "$img"
  done < <(yq -r '.applications[] | select(has("images")) | .images[]' "$ARTIFACTS_FILE")
fi

if [[ "$SKIP_LICENSES" != "1" ]]; then
  [[ -f "$LICENSE_FILE" ]] || {
    echo "License file not found: $LICENSE_FILE" >&2
    exit 1
  }
  if [[ -z "$KOMMANDER" ]]; then
    echo "Set KOMMANDER for \${kommander} expansion or keep $ARTIFACTS_FILE with kommander2-core-installer tag." >&2
    exit 1
  fi
  while IFS= read -r img; do
    add_ref "${img//\$\{kommander\}/$KOMMANDER}"
  done < <(yq -r '.resources[].container_image' "$LICENSE_FILE")
fi

mapfile -t refs < <(printf '%s\n' "${refs[@]}" | sort -u)
if [[ ${#refs[@]} -eq 0 ]]; then
  echo "No container references to validate." >&2
  exit 1
fi

failed=()
for ref in "${refs[@]}"; do
  if ! crane digest "$ref" >/dev/null 2>&1; then
    failed+=("$ref")
  fi
done

if [[ ${#failed[@]} -gt 0 ]]; then
  printf '%s\n' "${failed[@]}" | sort -u
  exit 1
fi
