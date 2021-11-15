#!/usr/bin/env bash
set -euxo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
LATEST_FLUX_VERSION="$(gh api -X GET "repos/fluxcd/flux2/releases" --jq '.[0].tag_name|sub("^v"; "")')"
readonly LATEST_FLUX_VERSION
CURRENT_FLUX_VERSION=$(find "${REPO_ROOT}/services/kommander-flux" -maxdepth 1 -regextype sed -regex '.*/[0-9]\+.[0-9]\+.[0-9]\+' -printf "%f\n" | sort -V | head -1)
readonly CURRENT_FLUX_VERSION

function update_flux() {
    readonly BRANCH_NAME="flux-update/${LATEST_FLUX_VERSION}"
    git checkout -b "${BRANCH_NAME}"

    asdf install flux2 "${LATEST_FLUX_VERSION}"
    asdf local flux2 "${LATEST_FLUX_VERSION}"

    mkdir -p "$REPO_ROOT/services/kommander-flux/$LATEST_FLUX_VERSION"
    pushd "$REPO_ROOT/services/kommander-flux/$LATEST_FLUX_VERSION"
    ls ..
    cp -a ../"$CURRENT_FLUX_VERSION"/* .
    rm -r ../"$CURRENT_FLUX_VERSION"
    rm templates/*
    flux install -n kommander-flux --export > templates/flux.yaml
    cp "$REPO_ROOT/hack/flux/flux-update-kustomization.yaml" templates/kustomization.yaml
    cp "$REPO_ROOT"/hack/flux/templates/* templates/
    kustomize build --output templates templates
    rm templates/flux.yaml templates/kustomization.yaml
    pushd "templates"
    kustomize create --autodetect

    git add .

    if [[ -z "$(git config user.email 2>/dev/null || true)" ]]; then
        git config user.email "ci@mesosphere.com"
        git config user.name "CI"
    fi

    readonly COMMIT_MSG="feat: Upgrade flux to ${LATEST_FLUX_VERSION}"

    git commit -m "${COMMIT_MSG}"

    git push --set-upstream origin "${BRANCH_NAME}"

    git fetch origin main
    gh pr create --base main --fill --head "${BRANCH_NAME}" -t "${COMMIT_MSG}" -l ready-for-review
}

if [ "${CURRENT_FLUX_VERSION}" == "${LATEST_FLUX_VERSION}" ]; then
  echo "Flux version is up to date - nothing to do"
  exit 0
fi

echo "Updating flux version from ${CURRENT_FLUX_VERSION} to ${LATEST_FLUX_VERSION}"

update_flux
