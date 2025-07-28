VERSION="v0.2.3"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

REPO_ROOT="$(git rev-parse --show-toplevel)"

gh -R nutanix-cloud-native/flux-oci-mirror release download "$VERSION" --dir "$TMP_DIR" --pattern '*.yaml'

CURRENT_FLUX_VERSION=$(find "${REPO_ROOT}/applications/kommander-flux" -maxdepth 1 -regextype sed -regex '.*/[0-9]\+.[0-9]\+.[0-9]\+' -printf "%f\n" | sort -V | head -1)

cp $TMP_DIR/flux-oci-mirror-cert-manager.yaml "$REPO_ROOT/applications/kommander-flux/$CURRENT_FLUX_VERSION/mirror/flux-oci-mirror.yaml"
