set unstable
# Recipes for releasing various artifacts related to kommander-applications

git_tag := env_var_or_default("GIT_TAG", "v0.0.0")

registry := "ghcr.io"
org_name := "mesosphere"
repository := org_name / "kommander-applications"
include_file := justfile_directory() / ".include-airgapped"
exclude_file := justfile_directory() / ".exclude-airgapped"
git_operator_version := env("GIT_OPERATOR_VERSION", "latest")
server_repository := registry / org_name / "kommander-applications-server"

s3_path := "dkp" / git_tag
s3_bucket := "downloads.mesosphere.io"
s3_uri := "s3://" + s3_bucket / s3_path
s3_acl := "bucket-owner-full-control"
archive_name := "kommander-applications-" + git_tag+ ".tar.gz"
full_archive_name := "kommander-applications-full-" + git_tag + ".tar.gz"
published_url := "https://downloads.d2iq.com" / s3_path / archive_name
full_published_url := "https://downloads.d2iq.com" / s3_path / full_archive_name

release publish="true" tmp_dir=`mktemp --directory` full_tmp_dir=`mktemp --directory`: (_prepare-archive tmp_dir) (_prepare-archive-full full_tmp_dir) && _cleanup
    #!/usr/bin/env bash
    set -euox pipefail
    if {{ publish }}; then
      aws s3 cp --acl {{ s3_acl }} {{ archive_name }} {{ s3_uri }}/{{ archive_name }}
      echo "Published to {{ published_url }}"

      aws s3 cp --acl {{ s3_acl }} {{ full_archive_name }} {{ s3_uri }}/{{ full_archive_name }}
      echo "Published to {{ full_published_url }}"
    else
      echo "Skipping publish"
    fi

release-oci publish="true" tmp_dir=`mktemp --directory`: (_prepare-files-for-a-bundle tmp_dir)
    #!/usr/bin/env bash
    set -euox pipefail
    cd {{ tmp_dir }}
    if {{ publish }}; then
      oras push --username "${DOCKER_USERNAME}" --password "${DOCKER_PASSWORD}" --verbose {{ registry }}/{{ repository }}:{{ git_tag }} .
    else
      echo "Skipping publish"
    fi

[script]
release-platform-collection-artifact publish="true": install-tools
  {{nkp_cli_bin}} create catalog-collection-artifact --repo-dir {{ justfile_directory() }} --collection-tag {{ git_tag }} --collection-annotations com.nutanix.nkp.catalog.artifact-type=platform-catalog-collection --collection-annotations org.opencontainers.image.source=https://github.com/mesosphere/kommander-applications -o {{ justfile_directory() }}/kommander-applications-platform-collection-{{ git_tag }}.tar
  if {{ publish }}; then
    {{nkp_cli_bin}} push bundle --bundle {{ justfile_directory() }}/kommander-applications-platform-collection-{{ git_tag }}.tar --to-registry {{ registry }}/{{ org_name }}
  else
    echo "Skipping publish"
  fi

_prepare-archive dir: (_prepare-files-for-a-bundle dir)
    tar -cvzf {{ justfile_directory() }}/{{ archive_name }} -C {{ dir }} .

_prepare-archive-full full_dir: (_prepare-files-for-full-bundle full_dir)
	tar -cvzf {{ justfile_directory() }}/{{ full_archive_name }} -C {{ full_dir }} .

_cleanup:
    rm {{ archive_name }} {{ full_archive_name }}

_prepare-files-for-a-bundle output_dir:
    rsync --quiet --archive --recursive --files-from={{ include_file }} --exclude-from={{ exclude_file }} {{ justfile_directory() }} {{ output_dir }}

_prepare-files-for-full-bundle full_output_dir:
	rsync --quiet --archive --recursive --files-from={{ include_file }} {{ justfile_directory() }}/ {{ full_output_dir }}/
