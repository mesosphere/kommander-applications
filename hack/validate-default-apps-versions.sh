#!/usr/bin/env bash
set -euo pipefail

latestKommanderVersion=$(find services/kommander -maxdepth 1 -type d | sort | tail -1)

readarray apps < <(yq -e '.data["values.yaml"]' "$latestKommanderVersion/defaults/cm.yaml" | yq -e '.attached.prerequisites.defaultApps')

for app in "${apps[@]}"; do
  name=$(echo "$app" | yq -e 'keys | .[0]')
  version=$(echo "$app" | yq -e '.[]')
  if [ ! -d "services/$name/$version" ]; then
    echo "kommander is set to use $name in version $version which doesn't exist"
    exit 1
  fi
done
