set dotenv-load

registry := "registry-1.docker.io"
org_name := "mesosphere"
repository := org_name / "kommander-applications"
include_file := justfile_directory() / ".include-airgapped"
exclude_file := justfile_directory() / ".exclude-airgapped"
git_operator_version := env("GIT_OPERATOR_VERSION", "latest")

release-oci tag tmp_dir=`mktemp --directory`:
    rsync --info=name --archive --recursive --files-from={{ include_file }} --exclude-from={{ exclude_file }} {{ justfile_directory() }} {{ tmp_dir }}
    cd {{ tmp_dir }} && echo "${DOCKER_PASSWORD}" | oras push --password-stdin --username "${DOCKER_USERNAME}" --verbose {{ registry }}/{{ repository }}:{{ tag }} .

service_version:=`ls services/git-operator/ | grep -E "v?[[:digit:]]\.[[:digit:]]\.[[:digit:]]"`
service_dir:=justfile_directory() / "services/git-operator" / service_version

git-operator-fetch-manifests tmp_dir=`mktemp --directory`:
    flux pull artifact oci://docker.io/mesosphere/git-operator-manifests:{{ git_operator_version }} --output {{ tmp_dir }}
    # HACK: strip SHA off git-operator image
    kustomize build {{ tmp_dir }}/default | sed -r 's/(image\: docker\.io\/mesosphere\/git-operator\:v[0-9]+\.[0-9]+.[0-9]+)\@sha256\:.*?$/\1/g' >{{ service_dir }}/git-operator-manifests/all.yaml
    [ -z "$(git diff --name-only services/git-operator)" ] || echo -e '\n\n\nWARNING: Git Operator manifests have changed!\nEdit {{ service_dir }}/additional-images.txt to ensure additional images are up to date.\n\n'
