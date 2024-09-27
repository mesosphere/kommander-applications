set dotenv-load

git_tag := env_var_or_default("GIT_TAG", "v0.0.0")

registry := "docker.io"
org_name := "mesosphere"
repository := org_name / "kommander-applications"
include_file := justfile_directory() / ".include-airgapped"
exclude_file := justfile_directory() / ".exclude-airgapped"
git_operator_version := env("GIT_OPERATOR_VERSION", "latest")

s3_path := "dkp" / git_tag
s3_bucket := "downloads.mesosphere.io"
s3_uri := "s3://" / s3_bucket / s3_path
s3_acl := "bucket-owner-full-control"
archive_name := "kommander-applications-" + git_tag+ ".tar.gz"
published_url := "https://downloads.d2iq.com" / s3_path / archive_name

release tmp_dir=`mktemp --directory`: (_prepare-archive tmp_dir)
    aws s3 cp --acl {{ s3_acl }} {{ archive_name }} {{ s3_uri }}
    @echo "Published to {{ published_url }}"

release-oci tmp_dir=`mktemp --directory`: (_prepare-files-for-a-bundle tmp_dir)
    cd {{ tmp_dir }} && echo "${DOCKER_PASSWORD}" | oras push --password-stdin --username "${DOCKER_USERNAME}" --verbose {{ registry }}/{{ repository }}:{{ git_tag }} .

service_version:=`ls services/git-operator/ | grep -E "v?[[:digit:]]\.[[:digit:]]\.[[:digit:]]"`
service_dir:=justfile_directory() / "services/git-operator" / service_version

git-operator-fetch-manifests tmp_dir=`mktemp --directory`:
    flux pull artifact oci://docker.io/mesosphere/git-operator-manifests:{{ git_operator_version }} --output {{ tmp_dir }}
    # HACK: strip SHA off git-operator image
    kustomize build {{ tmp_dir }}/default | sed -r 's/(image\: docker\.io\/mesosphere\/git-operator\:v[0-9]+\.[0-9]+.[0-9]+)\@sha256\:.*?$/\1/g' >{{ service_dir }}/git-operator-manifests/all.yaml
    [ -z "$(git diff --name-only services/git-operator)" ] || echo -e '\n\n\nWARNING: Git Operator manifests have changed!\nEdit {{ service_dir }}/additional-images.txt to ensure additional images are up to date.\n\n'

_prepare-archive dir: (_prepare-files-for-a-bundle dir)
    tar -cvzf {{ justfile_directory() }}/{{ archive_name }} -C {{ dir }} .

_cleanup:
    rm {{ archive_name }}

_prepare-files-for-a-bundle output_dir:
    rsync --archive --recursive --files-from={{ include_file }} --exclude-from={{ exclude_file }} {{ justfile_directory() }} {{ output_dir }}
    yq 'del(.resources[] | select(. == "ai-navigator-repos.yaml"))' --inplace {{ output_dir }}/common/helm-repositories/kustomization.yaml
