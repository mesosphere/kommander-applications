#!/usr/bin/env bash
set -euo pipefail

latestKommanderVersion=$(find "services/kommander" -maxdepth 1 -type d -regextype sed -regex '.*/[0-9]\+.[0-9]\+.[0-9]\+' -printf "%f\n" | sort --version-sort | tail -1)

validate_apps_in_yaml_path() {
  readarray apps < <(yq -e '.data["values.yaml"]' "services/kommander/$latestKommanderVersion/defaults/cm.yaml" | yq -e "$1")

  for app in "${apps[@]}"; do
    name=$(echo "$app" | yq -e 'keys | .[0]')
    version=$(echo "$app" | yq -e '.[]')
    if [ ! -d "services/$name/$version" ]; then
      echo "app $name in version $version specified in $1 doesn't exist"
      exit 1
    fi
  done
}

validate_apps_in_yaml_path ".attached.prerequisites.defaultApps"
validate_apps_in_yaml_path ".kommander-licensing.defaultEnterpriseApps"
