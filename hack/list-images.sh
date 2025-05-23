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

declare -rx patch=unused

while IFS= read -r repofile; do
  envsubst -no-unset -no-digit -i "${repofile}" | \
    gojq --yaml-input --raw-output 'select(.spec.url != null) | (.metadata.name | gsub("\\."; "-"))+" "+.spec.url' | \
    xargs --max-lines=1 --no-run-if-empty -- helm repo add --force-update >&2
done < <(grep --recursive --exclude-dir=apptests --max-count=1 --files-with-matches '^kind: HelmRepository')

helm repo update >&2

# Dummy variables to satisfy substitution vars used by Flux. Almost all of these do not affect the image being bundled,
# hence have values such as "unused" or are actually empty.
# If a substitution var is missed here, this script will fail below because `envsubst -no-unset` flag ensures that all
# necessary variables are set. In that case, the missing variables should be evaluated and added to this list as
# appropriate.
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
            kommanderAppManagementConfigAPIImageRepository='' \
            tfaName=unused \
            notPopulatedAnywhereAsThisIsOnlyForAirgappedBundle=unused \
            caIssuerName=unused \
            CLUSTER_ID=unused \
            kubecostClusterMode=unused \
            kommanderChartVersion="${kommanderChartVersion:-}"

IMAGES_FILE="$(realpath "$(mktemp .helm-list-images-XXXXXX)")"
readonly IMAGES_FILE
trap_add "rm --force ${IMAGES_FILE}" EXIT

# extracts list of referenced configmap and its key as
# helm release values, returns in a format "cm_name/values_key.yaml"
# @param 1 helm release file
hr:valuesFrom () {
  envsubst -no-unset -no-digit -i "$1" | \
    gojq --yaml-input --raw-output '
        select(.kind == "HelmRelease") | .spec.valuesFrom // [] | .[] |
        select(.kind == "ConfigMap") | [.name, .valuesKey // "values.yaml"] | join("/")
    '
}

# find cm map by "cm_name/values_key.yaml" in given directory echo
# values.
# @param 1 cm_name/values_key.yaml
# @param 2 path to a directory directory with configmaps
cm:find () {
  local cm_name=$1
  local dir=$2

  if [ ! -f "${dir}/kustomization.yaml" ]; then
    return
  fi

  # shellcheck disable=SC2016
  kubectl kustomize "${dir}" | \
    flux envsubst | \
    gojq --yaml-input --raw-output --arg name "${cm_name%/*}" --arg key "${cm_name##*/}" '
      select(.kind == "ConfigMap") | select(.metadata.name == $name) | .data[$key]
    '
}

for dir in $(find . -path "./apptests/*" -prune -o -type f -name "*.yaml" -print0 | xargs --null --max-lines=1 --no-run-if-empty -- grep --files-with-matches '^kind: HelmRelease' | grep --only-matching "\(.*\)/" | sort --unique); do
  pushd "${dir}" &>/dev/null

  while IFS= read -r hr; do
    pushd "$(dirname "${hr}")" &>/dev/null
    extra_args=()
    defaults_cm=""
    if [ -f 'defaults/cm.yaml' ]; then
      defaults_cm='defaults/cm.yaml'
    elif [ -f '../defaults/cm.yaml' ]; then
      defaults_cm='../defaults/cm.yaml'
    fi
    if [ -n "${defaults_cm}" ]; then
      temp_values="$(mktemp .helm-list-images-XXXXXX)"
      trap_add "rm --force $(realpath "${temp_values}")" EXIT
      envsubst -no-unset -no-digit -i "${defaults_cm}" | gojq --yaml-input --raw-output '.data.["values.yaml"]' | sed '/---/d' >"${temp_values}"
      extra_args+=('--values' "${temp_values}")
    fi

    defaults_dir=""
    if [ -d 'defaults' ]; then
      defaults_dir='defaults/'
    elif [ -d '../defaults' ]; then
      defaults_dir='../defaults/'
    fi

    if [ -n "$defaults_dir" ]; then
      for cm in $(hr:valuesFrom "$(basename "${hr}")")
      do
        temp_values="$(mktemp .helm-list-images-XXXXXX)"
        trap_add "rm --force $(realpath "${temp_values}")" EXIT
        cm:find "$cm" "$defaults_dir" > "${temp_values}"
        extra_args+=('--values' "${temp_values}")
      done
    fi

    if [ -f 'list-images-values.yaml' ]; then
      extra_args+=('--values' 'list-images-values.yaml')
    fi
    if [ -f 'extra-images.txt' ]; then
      extra_args+=('--extra-images-file' 'extra-images.txt')
    fi

    >&2 echo -e " + ${dir}${hr}\n"
    envsubst -no-unset -no-digit -i "$(basename "${hr}")" | \
      gojq --yaml-input --raw-output --arg repoRoot "${REPO_ROOT}" \
        $'select(.spec.chart.spec.sourceRef.name != null) |
          if .spec.chart.spec.sourceRef.kind == "HelmRepository" then
            (.spec.chart.spec.sourceRef.name | gsub("\\\."; "-"))+"/"+.spec.chart.spec.chart+" --chart-version="+.spec.chart.spec.version
          elif .spec.chart.spec.sourceRef.kind == "GitRepository" then
            $repoRoot+"/"+.spec.chart.spec.chart
          end' | \
      xargs --max-lines=1 --no-run-if-empty -- helm list-images --unique "${extra_args[@]}" | >&2 tee -a "${IMAGES_FILE}"
      >&2 echo
    popd &>/dev/null
  done < <(grep --recursive --max-count=1 --files-with-matches '^kind: HelmRelease')
  popd &>/dev/null
done

# These services use raw manifests rather than Helm charts so list the images directly from the manifests.
# If more raw manifest services are added, then they should be added to the list of paths below.
{
  gojq --yaml-input --raw-output 'select(.kind | test("^(?:Deployment|Job|CronJob|StatefulSet|DaemonSet)$")) |
                                  (.spec.template.spec // .spec.jobTemplate.spec.template.spec) |
                                  (select(.containers != null) | .containers[].image), (select(.initContainers != null) | .initContainers[].image)' \
                                  ./services/kommander-flux/*/templates/* \
                                  ./services/kube-prometheus-stack/*/etcd-metrics-proxy/* \
  # process git operator separately
  gojq --yaml-input --raw-output 'select(.kind | test("^(?:Deployment|Job|StatefulSet|DaemonSet)$")) |
                                  .spec.template.spec |
                                  (select(.containers != null) | .containers[].image), (select(.initContainers != null) | .initContainers[].image)' \
  				./services/git-operator/*/git-operator-manifests/* \
  # we patch the cronjob image in this kustomization
  gojq --yaml-input --raw-output 'select(.kind | test("^(?:Kustomization)$")) | .images | map("\(.name):\(.newTag)") | .[]' \
          ./services/git-operator/*/kustomization.yaml
} >>"${IMAGES_FILE}"

# Ensure that all images are fully qualified to ensure uniqueness of images in the image bundle.
sed --expression='s|^docker.io/||' \
    --expression='s|\(^[^/]\+$\)|library/\1|' \
    --expression='/^registry\.k8s\.io\//b; s|\(^[^/]\+/[^/]\+$\)|docker.io/\1|' \
    --expression='s|\(^[^:]\+:\?$\)|\1:latest|' \
    --expression='/^[[:space:]]*$/d' \
    --expression='/ai-navigator-/d' \
    --expression='/nkp-pulse-/d' \
    "${IMAGES_FILE}" | \
  sort --unique
