app_version:=`ls applications/git-operator/ | grep -E "^v?[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+$"`
app_dir:=justfile_directory() / "applications/git-operator" / app_version

release-server publish="true" tmp_dir=`mktemp --directory`: (_prepare-git-repository tmp_dir)
    cp -r {{ tmp_dir }} ./server/data/
    cd ./server && docker buildx build . --tag {{ server_repository }}:{{ git_tag }}
    rm -rf ./server/data/
    if {{ publish }}; then docker push {{ server_repository }}:{{ git_tag }}; fi

git-operator-fetch-manifests tmp_dir=`mktemp --directory`:
    flux pull artifact oci://docker.io/mesosphere/git-operator-manifests:{{ git_operator_version }} --output {{ tmp_dir }}
    # HACK: strip SHA off git-operator image
    kustomize build {{ tmp_dir }}/default | sed -r 's/(image\: docker\.io\/mesosphere\/git-operator\:v[0-9]+\.[0-9]+.[0-9]+)\@sha256\:.*?$/\1/g' >{{ app_dir }}/git-operator-manifests/all.yaml
    [ -z "$(git diff --name-only applications/git-operator)" ] || echo -e '\n\n\nWARNING: Git Operator manifests have changed!\nEdit {{ app_dir }}/additional-images.txt to ensure additional images are up to date.\n\n'

_prepare-git-repository output_dir tmp_dir_for_cloning=`mktemp --directory`:
    cd {{ output_dir }} && git init --bare --initial-branch=main
    git clone {{ output_dir }} {{ tmp_dir_for_cloning }}
    just --justfile {{ justfile() }} --working-directory {{ invocation_directory() }} _prepare-files-for-a-bundle {{ tmp_dir_for_cloning }}
    cd {{ tmp_dir_for_cloning }} && git add . && git commit --no-gpg-sign --message "initial commit" && git push origin main
