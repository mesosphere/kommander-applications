registry := "registry-1.docker.io"
org_name := "mesosphere"
repository := org_name / "kommander-applications"
include_file := justfile_directory() / ".include-airgapped"
exclude_file := justfile_directory() / ".exclude-airgapped"

release-oci tag tmp_dir=`mktemp --directory`:
    rsync --info=name --archive --recursive --files-from={{ include_file }} --exclude-from={{ exclude_file }} {{ justfile_directory() }} {{ tmp_dir }}
    cd {{ tmp_dir }} && echo "${DOCKER_PASSWORD}" | oras push --password-stdin --username "${DOCKER_USERNAME}" --verbose {{ registry }}/{{ repository }}:{{ tag }} .
