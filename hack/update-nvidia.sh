#!/bin/bash
set -e
readarray -t images < <(gojq -c '.[]' "${1}") #convert to bash arrays and remove trailing newline for each line (if any)
new_images=() # we need to create new array to strip the new line when converting back to json file

IFS=$'\n' #read til newline
for image in "${images[@]}"; do
  new_image=$(echo "${image}" | xargs)
  if [[ ${new_image} == *"nvidia"* ]]; then
    # dcgm-exporter:3.1.3-3.1.2-ubuntu20.04
    full_component=$(echo "${new_image}" | rev | cut -d'/' -f 1 | rev)
    component=$(echo "${full_component}" | cut -d':' -f1)
    version=$(echo "${full_component}" | cut -d':' -f2)
    case $component in
      gpu-feature-discovery)
        new_version=$(tail --lines=+8 "${2}"| gojq --yaml-input '.gfd.version' | xargs)
        new_image="${new_image//"${version}"/"${new_version}"}"
      ;;
      dcgm)
        new_version=$(tail --lines=+8 "${2}"| gojq --yaml-input '.dcgm.version'| xargs)
        new_image="${new_image//"${version}"/"${new_version}"}"
      ;;
      dcgm-exporter)
        new_version=$(tail --lines=+8 "${2}"| gojq --yaml-input '.dcgmExporter.version'| xargs)
        new_image="${new_image//"${version}"/"${new_version}"}"
      ;;
      gpu-operator-validator)
        new_version=$(tail --lines=+8 "${2}"| gojq --yaml-input '.validator.version'| xargs)
        new_image="${new_image//"${version}"/"${new_version}"}"
      ;;
      devicePlugin)
        new_version=$(tail --lines=+8 "${2}"| gojq --yaml-input '.devicePlugin.config.version'| xargs)
        new_image="${new_image//"${version}"/"${new_version}"}"
      ;;
      *)
        echo "skipping component ${component}"
      ;;
    esac
  fi
  echo "adding ${new_image}"
  new_images+=("${new_image}")
done
unset IFS
truncate -s 0 "${1}"
printf '%s\n' "${new_images[@]}" | gojq -R . | gojq -s .  >> "${1}"
