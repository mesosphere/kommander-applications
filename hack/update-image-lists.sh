#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

trap_add() {
  local -r sig="${2:?Signal required}"
  local -r hdls="$(trap -p "${sig}" | cut -f2 -d \')"
  # shellcheck disable=SC2064 # Quotes are required here to properly expand when adding the new trap.
  trap "${hdls}${hdls:+;}${1:?Handler required}" "${sig}"
}

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
pushd "${REPO_ROOT}" &>/dev/null

while IFS= read -r repofile; do
  envsubst <"${repofile}" | \
    gojq --yaml-input --raw-output 'select(.spec.url != null) | (.metadata.name | gsub("\\."; "-"))+" "+.spec.url' | \
    xargs -l1 --no-run-if-empty -- helm repo add --force-update
done < <(grep -R -m1 -l '^kind: HelmRepository')

# Dummy variables
declare -rx releaseNamespace=unused \
            RES="" \
            targetNamespace=unused \
            generatedName=unused \
            name=unused \
            namespace=unused \
            clusterID=unused \
            prometheusService=unused \
            releaseName=unused \
            valuesFrom=unused \
            workspaceNamespace=unused \
            chartmuseumAdminCredentialsSecret=unused \
            certificateIssuerName=unused \
            verb=unused \
            objectRef=unused \
            user=unused \
            adminCredentialsSecret=unused \
            tlsCertificateSecret=unused \
            airgappedEnabled=true \
            kommanderAuthorizedlisterImageTag='' \
            kommanderAuthorizedlisterImageRepository='' \
            certificatesCAIssuerName=unused \
            caSecretNamespace=unused \
            kommanderControllerManagerImageTag='' \
            kommanderControllerManagerImageRepository='' \
            kommanderFluxNamespace=unused \
            kommanderGitCredentialsSecretName=unused \
            ageEncryptionSecretName=unused \
            ageEncryptionSecretKey=unused \
            kommanderControllerWebhookImageTag='' \
            kommanderControllerWebhookImageRepository='' \
            kommanderFluxOperatorManagerImageTag='' \
            kommanderFluxOperatorManagerImageRepository='' \
            certificatesIssuerName=unused \
            caSecretName=unused \
            kommanderLicensingControllerManagerImageTag='' \
            kommanderLicensingControllerManagerImageRepository='' \
            kommanderLicensingControllerWebhookImageTag='' \
            kommanderLicensingControllerWebhookImageRepository='' \
            kommanderAppManagementReplicas='' \
            kommanderAppManagementImageTag='' \
            kommanderAppManagementImageRepository='' \
            kommanderAppManagementKubetoolsImageRepository='' \
            kommanderAppManagementWebhookImageRepository='' \
            tfaName=unused

readonly IMAGES_FILE="${REPO_ROOT}/images.txt"
rm -f "${IMAGES_FILE}"

readonly METADATA_FILENAME='metadata.yaml'
for dir in $(find . -type f -name "${METADATA_FILENAME}" | grep -o "\(.*\)/" | sort -u); do
  pushd "${dir}" &>/dev/null

  while IFS= read -r hr; do
    pushd "$(dirname "${hr}")" &>/dev/null
    extra_args=()
    if [ -f 'defaults/cm.yaml' ]; then
      temp_values="$(mktemp .helm-list-images-XXXXXX)"
      trap_add "rm -f $(realpath "${temp_values}")" EXIT
      envsubst -no-unset -i defaults/cm.yaml | gojq --yaml-input -r '.data.["values.yaml"]' >"${temp_values}"
      extra_args+=('--values' "${temp_values}")
    fi

    if [ -f 'list-images-values.yaml' ]; then
      extra_args+=('--values' 'list-images-values.yaml')
    fi
    if [ -f 'extra-images.txt' ]; then
      extra_args+=('--extra-images-file' 'extra-images.txt')
    fi

    envsubst -no-unset -i "$(basename "${hr}")" | \
      gojq --yaml-input --raw-output 'select(.spec.chart.spec.sourceRef.name != null) |
                                      select(.spec.chart.spec.sourceRef.kind == "HelmRepository") |
                                      (.spec.chart.spec.sourceRef.name | gsub("\\."; "-"))+"/"+.spec.chart.spec.chart+" --chart-version="+.spec.chart.spec.version' | \
      xargs -l1 --no-run-if-empty -- helm list-images --unique "${extra_args[@]}" >>"${IMAGES_FILE}"
    popd &>/dev/null
  done < <(grep -R -m1 -l '^kind: HelmRelease')
  popd &>/dev/null
done

sort -uo "${IMAGES_FILE}"{,}
