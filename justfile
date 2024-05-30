repository := "mesosphere"
include_file := justfile_directory() / ".include-airgapped"
exclude_file := justfile_directory() / ".exclude-airgapped"

release-oci tag tmp_dir=`mktemp --directory`:
    rsync --info=name --archive --recursive --files-from={{ include_file }} --exclude-from={{ exclude_file }} {{ justfile_directory() }} {{ tmp_dir }}
    echo "${DOCKER_PASSWORD}" | oras push --password-stdin --user "${DOCKER_USERNAME}" --verbose {{ repository }}/kommander-applications:{{ tag }} {{ tmp_dir }}
