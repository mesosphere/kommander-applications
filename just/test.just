test-server:
    #!/usr/bin/env bash
    set -euox pipefail
    CONTAINER_ID=$(just --justfile {{ justfile() }} --working-directory {{ invocation_directory() }} _run_server 2>&1 | tail -n 1)
    trap "docker kill ${CONTAINER_ID}" EXIT
    curl --no-progress-meter --output /dev/null --retry-connrefused --retry 5 --retry-delay 3 http://localhost:5000/{{ archive_name }}

_run_server: (release-server "false")
    docker run \
    --env DUFS_TLS_CERT \
    --env DUFS_TLS_KEY \
    --network=host \
    --detach \
    {{ server_docker_repository }}:{{ git_tag }}
