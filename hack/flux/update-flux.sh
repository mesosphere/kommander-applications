#!/usr/bin/env bash
set -euxo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
LATEST_FLUX_VERSION="$(gh api -X GET "repos/fluxcd/flux2/releases" --jq '.[0].tag_name|sub("^v"; "")')"
readonly LATEST_FLUX_VERSION
CURRENT_FLUX_VERSION=$(find "${REPO_ROOT}/applications/kommander-flux" -maxdepth 1 -regextype sed -regex '.*/[0-9]\+.[0-9]\+.[0-9]\+' -printf "%f\n" | sort -V | head -1)
readonly CURRENT_FLUX_VERSION

# Activate devbox shell if available
if command -v devbox &> /dev/null; then
    eval "$(devbox shellenv)"
fi

function check_remote_branch() {
    if [[ -n $(git ls-remote --exit-code --heads https://github.com/mesosphere/"$1".git "$2") ]]; then
        echo "Flux update PR is already up!"
        exit 0
    fi
}

function ensure_flux_version() {
    local_flux_version=$(flux --version)
    if [[ "$local_flux_version" == "$LATEST_FLUX_VERSION" ]]; then
        echo "updating flux to ${local_flux_version}"
    else
        echo "flux ${LATEST_FLUX_VERSION} not available in devbox, the latest available is ${local_flux_version}"
        echo "running devbox update"
        devbox update
    fi
}

function update_version_directory() {
    local version_dir="$REPO_ROOT/applications/kommander-flux/$LATEST_FLUX_VERSION"
    local current_version_dir="$REPO_ROOT/applications/kommander-flux/$CURRENT_FLUX_VERSION"

    if [ -d "$current_version_dir" ] && [ "$CURRENT_FLUX_VERSION" != "$LATEST_FLUX_VERSION" ]; then
        mv "$current_version_dir" "$version_dir"
    fi
    echo "$version_dir"
}

function update_ocirepository_and_helmrelease() {
    local version_dir="$1"
    local version="$2"

    # Convert version format for OCIRepository name (2.16.1 -> 2-16-1)
    local ocirepo_name_version=$(echo "$version" | tr '.' '-')
    local ocirepo_name="nkp-flux-${ocirepo_name_version}"

    local helmrelease_file="$version_dir/helmrelease/helmrelease.yaml"
    local cm_file="$version_dir/helmrelease/cm.yaml"

    # Update OCIRepository
    yq eval -i ".spec.ref.tag = \"${version}\"" "$helmrelease_file" -d 0
    yq eval -i ".metadata.name = \"${ocirepo_name}\"" "$helmrelease_file" -d 0
    # Update HelmRelease
    yq eval -i ".spec.chartRef.name = \"${ocirepo_name}\"" "$helmrelease_file" -d 1
    yq eval -i ".spec.valuesFrom[0].name = \"kommander-flux-${version}-config-defaults\"" "$helmrelease_file" -d 1

    # Update ConfigMap
    yq eval -i ".metadata.name = \"kommander-flux-${version}-config-defaults\"" "$cm_file"

    echo "$ocirepo_name"
}

function generate_bootstrap_manifests() {
    local version_dir="$1"
    local bootstrap_dir="$REPO_ROOT/applications/kommander-flux"
    local helmrelease_dir="$version_dir/helmrelease"
    local helmrelease_file="$helmrelease_dir/helmrelease.yaml"
    local cm_file="$helmrelease_dir/cm.yaml"
    local version=$(basename "$version_dir")

    # Extract OCIRepository document
    local ocirepo=$(yq eval 'select(.kind == "OCIRepository") | .' "$helmrelease_file" 2>/dev/null)

    # Set default values for variables
    export releaseNamespace="kommander-flux"
    export ociRegistryURL="oci://ghcr.io"

    # Process the OCIRepository to evaluate variables
    local temp_processed=$(mktemp)
    local temp_values=$(mktemp)
    trap 'rm -f "$temp_values" "$temp_processed" 2>/dev/null' RETURN

    echo "$ocirepo" | envsubst > "$temp_processed"

    # Extract OCI URL
    local chart_url=$(yq eval '.spec.url' "$temp_processed" 2>/dev/null)

    if [ -z "$chart_url" ] || [ "$chart_url" = "null" ]; then
        echo "Error: Could not extract URL from helmrelease.yaml"
        exit 1
    fi

    echo "Chart URL: $chart_url"
    echo "Version: $version"

    # Extract values from cm.yaml
    yq eval '.data."values.yaml"' "$cm_file" > "$temp_values"
    if [ ! -s "$temp_values" ]; then
        echo "Warning: Could not extract values from $cm_file"
        echo "" > "$temp_values"
    fi

    # Run helm template
    echo "Running helm template..."
    helm template flux2 "$chart_url" \
        --version "$version" \
        --namespace kommander-flux \
        --include-crds --no-hooks \
        -f "$temp_values" \
        > "$bootstrap_dir/bootstrap-flux.yaml"

    echo "Generated bootstrap-flux.yaml"

    # Update bootstrap kustomization.yaml
    local kustomization_file="$bootstrap_dir/kustomization.yaml"
    local ocirepo_name=$(yq eval '.metadata.name' "$temp_processed" 2>/dev/null)

    cat > "$kustomization_file" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: kommander-flux
resources:
  - bootstrap-flux.yaml
  - ./${version}
patches:
  - patch: |-
      \$patch: delete
      apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: ${ocirepo_name}
        namespace: \${releaseNamespace}
  - patch: |-
      \$patch: delete
      apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: kommander-flux
        namespace: \${releaseNamespace}
  - patch: |-
      \$patch: delete
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: kommander-flux-${version}-config-defaults
        namespace: \${releaseNamespace}
EOF

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
    local pr=$(gh pr create --base main --fill --head "${branch_name}" -t "${commit_msg}" -l ready-for-review -l ok-to-test -l slack-notify -l update-licenses)
    echo "${pr} is created"
}

function update_flux() {
    local branch_name="flux-update/${LATEST_FLUX_VERSION}"
    check_remote_branch "kommander-applications" "${branch_name}"
    git checkout -b "${branch_name}"

    ensure_flux_version

    local version_dir=$(update_version_directory)
    update_ocirepository_and_helmrelease "$version_dir" "$LATEST_FLUX_VERSION"
    generate_bootstrap_manifests "$version_dir"
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
