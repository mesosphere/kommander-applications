#!/usr/bin/env bash
# Verify every image / OCI reference from the catalog (artifacts_full.yaml) and from
# licenses.d2iq.yaml resolves in a registry (crane digest / manifest exists).
#
# Requires: yq, crane on PATH. Configure registry auth separately (docker login, crane auth).
#
# Env:
#   ARTIFACTS_FILE   (default: repo root artifacts_full.yaml)
#   LICENSE_FILE     (default: licenses.d2iq.yaml)
#   KOMMANDER        tag for ${kommander} in licenses (default: from artifacts)
#   PARALLEL         concurrent probes (default: 6)
#   SKIP_LICENSES=1  only check artifacts_full.yaml
#   SKIP_ARTIFACTS=1 only check licenses.d2iq.yaml
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACTS_FILE="${ARTIFACTS_FILE:-${ROOT}/artifacts_full.yaml}"
LICENSE_FILE="${LICENSE_FILE:-${ROOT}/licenses.d2iq.yaml}"
PARALLEL="${PARALLEL:-6}"
SKIP_LICENSES="${SKIP_LICENSES:-0}"
SKIP_ARTIFACTS="${SKIP_ARTIFACTS:-0}"

if ! command -v yq >/dev/null 2>&1; then
  echo "yq is required on PATH" >&2
  exit 1
fi
if ! command -v crane >/dev/null 2>&1; then
  echo "crane is required on PATH (https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md)" >&2
  exit 1
fi

derive_kommander_tag() {
  local artifacts="$1"
  if [[ ! -f "$artifacts" ]]; then
    return 1
  fi
  grep -m1 'docker.io/mesosphere/kommander2-core-installer:' "$artifacts" | sed 's/.*://' || return 1
}

if [[ -z "${KOMMANDER:-}" ]]; then
  KOMMANDER="$(derive_kommander_tag "$ARTIFACTS_FILE" || true)"
fi

raw_list="$(mktemp)"
trap 'rm -f "$raw_list"' EXIT

add_ref() {
  local raw="$1"
  [[ -z "$raw" ]] && return 0
  if [[ "$raw" == *'<registry-url>'* ]]; then
    return 0
  fi
  if [[ "$raw" == *"\${"* ]] || [[ "$raw" == *"\`"* ]]; then
    return 0
  fi
  local norm="$raw"
  if [[ "$norm" == oci://* ]]; then
    norm="${norm#oci://}"
  fi
  [[ -z "$norm" ]] && return 0
  # docker.io shorthand: if the first path segment is not a registry host (no '.'),
  # prefix docker.io/ so crane resolves e.g. goharbor/harbor-core:tag like Docker CLI.
  if [[ "$norm" == */*:* ]]; then
    local first="${norm%%/*}"
    if [[ "$first" != *.* ]] && [[ "$first" != localhost ]] && [[ "$first" != localhost:* ]]; then
      norm="docker.io/${norm}"
    fi
  fi
  printf '%s\n' "$norm" >>"$raw_list"
}

if [[ "$SKIP_ARTIFACTS" != "1" && -f "$ARTIFACTS_FILE" ]]; then
  while IFS= read -r img; do
    add_ref "$img"
  done < <(yq -r '.applications[] | select(has("images")) | .images[]' "$ARTIFACTS_FILE")
elif [[ "$SKIP_ARTIFACTS" != "1" ]]; then
  echo "Artifacts file not found: $ARTIFACTS_FILE" >&2
  exit 1
fi

if [[ "$SKIP_LICENSES" != "1" ]]; then
  if [[ ! -f "$LICENSE_FILE" ]]; then
    echo "License file not found: $LICENSE_FILE" >&2
    exit 1
  fi
  if [[ -z "${KOMMANDER:-}" ]]; then
    echo "Set KOMMANDER for \${kommander} expansion or keep $ARTIFACTS_FILE with kommander2-core-installer tag." >&2
    exit 1
  fi
  while IFS= read -r img; do
    expanded="${img//\$\{kommander\}/$KOMMANDER}"
    add_ref "$expanded"
  done < <(yq -r '.resources[].container_image' "$LICENSE_FILE")
fi

if [[ ! -s "$raw_list" ]]; then
  echo "No container references to validate (check SKIP_* flags and inputs)." >&2
  exit 1
fi

mapfile -t images < <(sort -u "$raw_list")
rm -f "$raw_list"
trap - EXIT

if [[ -n "${KOMMANDER:-}" ]]; then
  echo "Using KOMMANDER=${KOMMANDER} for license substitution where applicable"
fi
echo "Validating ${#images[@]} unique registry references (catalog + licenses)..."

crane_probe() {
  local ref="$1"
  if crane digest "$ref" >/dev/null 2>&1; then
    echo "OK:${ref}"
  else
    echo "MISS:${ref}"
  fi
}

export -f crane_probe

mapfile -t results < <(printf '%s\n' "${images[@]}" | xargs -n1 -P"$PARALLEL" bash -c "crane_probe \"\$1\"" bash)

fail_file="$(mktemp)"
trap 'rm -f "$fail_file"' EXIT

while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  case "$line" in
    OK:*)
      echo "$line"
      ;;
    MISS:*)
      echo "$line" >&2
      echo "${line#MISS:}" >>"$fail_file"
      ;;
  esac
done <<< "$(printf '%s\n' "${results[@]}")"

if [[ -s "$fail_file" ]]; then
  nfail="$(sort -u "$fail_file" | wc -l | tr -d ' ')"
  echo "" >&2
  echo "${nfail} reference(s) could not be resolved (missing manifest, tag, or registry auth):" >&2
  sort -u "$fail_file" >&2
  exit 1
fi

echo "All ${#images[@]} upstream references resolved successfully."
