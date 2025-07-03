#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(realpath "$(dirname "${SCRIPT_DIR}")")"
readonly REPO_ROOT
pushd "${REPO_ROOT}" &>/dev/null

trap_add() {
  local -r sig="${2:?Signal required}"
  local -r hdls="$(trap -p "${sig}" | cut --fields=2 --delimiter=\')"
  # shellcheck disable=SC2064 # Quotes are required here to properly expand when adding the new trap.
  trap "${hdls}${hdls:+;}${1:?Handler required}" "${sig}"
}

# Dummy variables to satisfy substitution vars used by Flux. Almost all of these do not affect the image being bundled,
# hence have values such as "unused" or are actually empty.
# If a substitution var is missed here, this script will fail below because `envsubst -no-unset` flag ensures that all
# necessary variables are set. In that case, the missing variables should be evaluated and added to this list as
# appropriate.
declare -rx releaseNamespace=unused \
            kommanderChartVersion="${kommanderChartVersion:-}" \
            ociRegistryURL="${ociRegistryURL:-}"

IMAGES_FILE="$(realpath "$(mktemp .helm-list-images-XXXXXX)")"
readonly IMAGES_FILE
trap_add "rm --force ${IMAGES_FILE}" EXIT

for dir in $(find . -path "./apptests/*" -prune -o -type f -name "*.yaml" -print0 | xargs --null --max-lines=1 --no-run-if-empty -- grep --files-with-matches '^kind: OCIRepository' | grep --only-matching "\(.*\)/" | sort --unique); do
  pushd "${dir}" &>/dev/null
  while IFS= read -r ocirepo_path; do
    >&2 echo "+ ${dir}${ocirepo_path}"
     envsubst -no-unset -no-digit < "${ocirepo_path}" | \
      yq -r --no-doc 'select(.kind == "OCIRepository") | .spec.url + ":" + .spec.ref.tag' | \
      >&2 tee -a "${IMAGES_FILE}"
  done < <(grep --recursive --max-count=1 --files-with-matches '^kind: OCIRepository')
  popd &>/dev/null
done

sort --unique "$IMAGES_FILE" | sed 's|^oci://||'
