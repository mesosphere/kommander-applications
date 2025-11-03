#!/usr/bin/env bash
set -euxo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
LATEST_FLUX_CHART_VERSION="$(gh api -X GET "repos/fluxcd-community/helm-charts/tags?per_page=100" --jq '.[] | select(.name | test("^flux2-[0-9]+\\.[0-9]+\\.[0-9]+$")) | .name | sub("^flux2-"; "")' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)"
[ -z "$LATEST_FLUX_CHART_VERSION" ] && { echo "Error: Could not determine latest Flux chart version"; exit 1; }
readonly LATEST_FLUX_CHART_VERSION
LATEST_FLUX_VERSION="$(gh api -X GET "repos/fluxcd-community/helm-charts/contents/charts/flux2/Chart.yaml?ref=flux2-${LATEST_FLUX_CHART_VERSION}" --jq '.content' | base64 -d | yq eval '.appVersion' -)"
[ -z "$LATEST_FLUX_VERSION" ] && { echo "Error: Could not determine latest Flux version from Chart.yaml"; exit 1; }
readonly LATEST_FLUX_VERSION
CURRENT_FLUX_VERSION=""
for dir in "${REPO_ROOT}"/applications/kommander-flux/[0-9]*.[0-9]*.[0-9]*; do
    if [ -d "$dir" ]; then
        CURRENT_FLUX_VERSION=$(basename "$dir" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+$')
        break
    fi
done
[ -z "$CURRENT_FLUX_VERSION" ] && { echo "Error: Could not determine current Flux version"; exit 1; }
readonly CURRENT_FLUX_VERSION

function check_remote_branch() {
    if [[ -n $(git ls-remote --exit-code --heads https://github.com/mesosphere/"$1".git "$2") ]]; then
        echo "Flux update PR is already up!"
        exit 0
    fi
}

function update_version_directory() {
    local version_dir="$REPO_ROOT/applications/kommander-flux/$LATEST_FLUX_VERSION"
    local current_version_dir="$REPO_ROOT/applications/kommander-flux/$CURRENT_FLUX_VERSION"

    if [ -d "$current_version_dir" ]; then
        mv "$current_version_dir" "$version_dir"
    fi
    echo "$version_dir"
}

function update_ocirepository_and_helmrelease() {
    local version_dir="$1"
    local version="$2"

    local ocirepo_name="nkp-flux-${LATEST_FLUX_CHART_VERSION}"

    pushd "$version_dir/helmrelease" >/dev/null
    # Update OCIRepository
    yq eval-all -i '(select(.kind == "OCIRepository") | .spec.ref.tag) = "'"${LATEST_FLUX_CHART_VERSION}"'"' helmrelease.yaml
    yq eval-all -i '(select(.kind == "OCIRepository") | .metadata.name) = "'"${ocirepo_name}"'"' helmrelease.yaml
    # Update HelmRelease
    yq eval-all -i '(select(.kind == "HelmRelease") | .spec.chartRef.name) = "'"${ocirepo_name}"'"' helmrelease.yaml
    yq eval-all -i '(select(.kind == "HelmRelease") | .spec.valuesFrom[0].name) = "kommander-flux-'"${version}"'-config-defaults"' helmrelease.yaml
    # Update ConfigMap
    yq eval -i ".metadata.name = \"kommander-flux-${version}-config-defaults\"" cm.yaml
    popd >/dev/null
}

function generate_bootstrap_manifests() {
    local version_dir="$1"
    local chart_version="$2"
    local bootstrap_dir="$REPO_ROOT/applications/kommander-flux"
    local version
    version=$(basename "$version_dir")
    local chart_url

    pushd "$version_dir/helmrelease" >/dev/null
    chart_url=$(yq eval 'select(.kind == "OCIRepository") | .spec.url' helmrelease.yaml)
    popd >/dev/null

    local temp_values
    temp_values=$(mktemp)
    trap 'rm -f "$temp_values" 2>/dev/null' RETURN

    echo "Chart URL: $chart_url"
    echo "Version: $version"

    # Extract values from cm.yaml
    pushd "$version_dir/helmrelease" >/dev/null
    yq eval '.data."values.yaml"' cm.yaml > "$temp_values"
    popd >/dev/null
    if [ ! -s "$temp_values" ]; then
        echo "Warning: Could not extract values from $version_dir/helmrelease/cm.yaml"
        echo "" > "$temp_values"
    fi

    # Run helm template
    echo "Running helm template..."
    pushd "$bootstrap_dir" >/dev/null
    helm template flux2 "$chart_url" \
        --version "$chart_version" \
        --namespace kommander-flux \
        --include-crds --no-hooks \
        -f "$temp_values" \
        > bootstrap-flux.yaml
    popd >/dev/null

    echo "Generated bootstrap-flux.yaml"

    # Update bootstrap kustomization.yaml
    pushd "$bootstrap_dir" >/dev/null
    local temp_file
    temp_file=$(mktemp)
    sed "s|./${CURRENT_FLUX_VERSION}|./${version}|g" kustomization.yaml > "$temp_file"
    mv "$temp_file" kustomization.yaml
    popd >/dev/null

    echo "Updated bootstrap kustomization.yaml"
}

function create_pr() {
    local branch_name="$1"
    local version="$2"

    git add applications

    local commit_msg="feat: Upgrade flux to ${version}"
    git commit -m "${commit_msg}"

    git push --set-upstream origin "${branch_name}"

    git fetch origin main
    echo "$(gh pr create --base main --fill --head "${branch_name}" -t "${commit_msg}" -l ready-for-review -l ok-to-test -l slack-notify -l update-licenses) is created"
}

function update_flux() {
    local branch_name="flux-update/${LATEST_FLUX_VERSION}"
    check_remote_branch "kommander-applications" "${branch_name}"
    git checkout -b "${branch_name}"

    local version_dir
    version_dir=$(update_version_directory)
    update_ocirepository_and_helmrelease "$version_dir" "$LATEST_FLUX_VERSION"
    generate_bootstrap_manifests "$version_dir" "$LATEST_FLUX_CHART_VERSION"
    create_pr "$branch_name" "$LATEST_FLUX_VERSION"
}

# Main execution - only run if script is executed directly (not sourced)
if [ "${BASH_SOURCE[0]}" = "${0}" ] && [ -z "${_SOURCING_FUNCTIONS_ONLY:-}" ]; then
    if [ "${CURRENT_FLUX_VERSION}" == "${LATEST_FLUX_VERSION}" ]; then
        echo "Flux version is up to date - nothing to do"
        exit 0
    fi

    echo "Updating flux version from ${CURRENT_FLUX_VERSION} to ${LATEST_FLUX_VERSION}"
    update_flux
fi
