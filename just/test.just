test_container_name := "kommander-applications-server"

test-server: _run_server
    sleep 3 # wait for container
    git ls-remote git://localhost:12345/data

_run_server: (release-server "false")
    docker run \
    --name {{ test_container_name }} \
    --publish 12345:12345 \
    --detach \
    --user 65532:65532 \
    {{ server_repository }}:{{ git_tag }}

_kill_server:
    docker kill {{ test_container_name }}
