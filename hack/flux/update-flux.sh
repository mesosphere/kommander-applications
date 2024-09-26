#!/usr/bin/env bash
set -euxo pipefail
IFS=$'\n\t'

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
LATEST_FLUX_VERSION="$(gh api -X GET "repos/fluxcd/flux2/releases" --jq '.[0].tag_name|sub("^v"; "")')"
readonly LATEST_FLUX_VERSION
CURRENT_FLUX_VERSION=$(find "${REPO_ROOT}/services/kommander-flux" -maxdepth 1 -regextype sed -regex '.*/[0-9]\+.[0-9]\+.[0-9]\+' -printf "%f\n" | sort -V | head -1)
readonly CURRENT_FLUX_VERSION
KOMMANDER_REPO_PATH="${REPO_ROOT}/kommander" # Override in CI to path of kommander repository.

function check_remote_branch() {
    if [[ -n $(git ls-remote --exit-code --heads https://github.com/mesosphere/"$1".git "$2") ]]; then
        echo "Flux update PR is already up!"
        exit 0
    fi
}

function update_flux() {
    readonly BRANCH_NAME="flux-update/${LATEST_FLUX_VERSION}"
    check_remote_branch "kommander-applications" "${BRANCH_NAME}"
    git checkout -b "${BRANCH_NAME}"

    local_flux_version=$(flux --version)
    if [[ "$local_flux_version" == "$LATEST_FLUX_VERSION" ]]; then
      echo "updating flux to ${local_flux_version}"
    else
      echo "flux ${LATEST_FLUX_VERSION} not available in devbox, the latest available is ${local_flux_version}"
    fi

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
    yq e -i ".metadata.name=\"kommander-flux-$LATEST_FLUX_VERSION-d2iq-defaults\"" defaults/cm.yaml
    pushd "templates"
    kustomize create --autodetect
    popd && popd

    git add services

    readonly COMMIT_MSG="feat: Upgrade flux to ${LATEST_FLUX_VERSION}"

    git commit -m "${COMMIT_MSG}"

    git push --set-upstream origin "${BRANCH_NAME}"

    git fetch origin main
    KOMMANDER_APPLICATIONS_PR=$(gh pr create --base main --fill --head "${BRANCH_NAME}" -t "${COMMIT_MSG}" -l ready-for-review -l ok-to-test -l slack-notify -l update-licenses)
    readonly KOMMANDER_APPLICATIONS_PR
    echo "${KOMMANDER_APPLICATIONS_PR} is created"
}

function bump_kommander_repo_flux() {
    ls -latrh "${KOMMANDER_REPO_PATH}"
    if [ ! -d "${KOMMANDER_REPO_PATH}" ]; then
        echo "error: kommander repo path is invalid (set to \"${KOMMANDER_REPO_PATH}\"). skipping flux upgrade in kommander repo"
        return 0
    fi
    echo "kommander repo found at ${KOMMANDER_REPO_PATH} and attempting to create a flux bump PR"
    pushd "${KOMMANDER_REPO_PATH}"
    check_remote_branch "kommander" "${BRANCH_NAME}"
    git checkout -b "${BRANCH_NAME}"
    sed -i "s~KOMMANDER_APPLICATIONS_REF ?= main~KOMMANDER_APPLICATIONS_REF ?= ${BRANCH_NAME}~g" Makefile
    git add Makefile
    git commit -m "${COMMIT_MSG}"
    git push --set-upstream origin "${BRANCH_NAME}"
    git fetch origin main
    gh pr create --base main --fill --head "${BRANCH_NAME}" -t "${COMMIT_MSG}" -l copy-flux-manifests -l test/kuttl -l test/kuttl-multi-cluster -l test/airgapped -l test/license -l test/e2e -l ready-for-review -l stacked -b "Depends on ${KOMMANDER_APPLICATIONS_PR}"
    popd
}

if [ "${CURRENT_FLUX_VERSION}" == "${LATEST_FLUX_VERSION}" ]; then
  echo "Flux version is up to date - nothing to do"
  exit 0
fi

echo "Updating flux version from ${CURRENT_FLUX_VERSION} to ${LATEST_FLUX_VERSION}"

update_flux
bump_kommander_repo_flux
