registry := "registry-1.docker.io"
org_name := "mesosphere"
repository := org_name / "kommander-applications"
include_file := justfile_directory() / ".include-airgapped"
exclude_file := justfile_directory() / ".exclude-airgapped"

release-oci tag tmp_dir=`mktemp --directory`:
    rsync --info=name --archive --recursive --files-from={{ include_file }} --exclude-from={{ exclude_file }} {{ justfile_directory() }} {{ tmp_dir }}
    cd {{ tmp_dir }} && echo "${DOCKER_PASSWORD}" | oras push --password-stdin --username "${DOCKER_USERNAME}" --verbose {{ registry }}/{{ repository }}:{{ tag }} .

git-operator-fetch-manifests tmp_dir=`mktemp --directory`:
    flux pull artifact oci://docker.io/mesosphere/git-operator-manifests:latest --output {{ tmp_dir }}
    kustomize build {{ tmp_dir }}/default > {{ justfile_directory() }}/services/git-operator/0.1.0/git-operator-manifests/all.yaml
