#!/usr/bin/env bash
# Bloodhound: nkp validate catalog-repository (.bloodhound.yml) writes a temp artifact
# bundle, then hack/validate-upstream-container-images.sh probes registries (crane).
#
# Requires: nkp (NKP_BIN), Helm 3+, yq, crane on PATH.
# Env: NKP_BIN
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NKP_BIN="${NKP_BIN:-}"
if [[ -z "$NKP_BIN" || ! -x "$NKP_BIN" ]]; then
  if command -v nkp >/dev/null 2>&1; then
    NKP_BIN="$(command -v nkp)"
  else
    echo "Set NKP_BIN to the nkp executable (e.g. make installs .local/bin/nkp_*)" >&2
    exit 1
  fi
fi

if ! command -v helm >/dev/null 2>&1; then
  echo "error: helm 3+ is required on PATH for nkp validate catalog-repository" >&2
  exit 1
fi

BH_CONFIG="${ROOT}/.bloodhound.yml"
if [[ ! -f "$BH_CONFIG" ]]; then
  echo "Missing Bloodhound config: $BH_CONFIG" >&2
  exit 1
fi

TMP="$(mktemp "${TMPDIR:-/tmp}/bloodhound-artifacts.XXXXXX.yaml")"
trap 'rm -f "$TMP"' EXIT

echo "Bloodhound: nkp validate catalog-repository --config .bloodhound.yml → ${TMP}"
"$NKP_BIN" validate catalog-repository \
  --repo-dir "$ROOT" \
  --config "$BH_CONFIG" \
  --artifacts-output "$TMP"

export ARTIFACTS_FILE="$TMP"
exec "${ROOT}/hack/validate-upstream-container-images.sh"
