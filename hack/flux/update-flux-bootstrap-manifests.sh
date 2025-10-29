#!/usr/bin/env bash
set -eo pipefail

# Script to generate flux-all.yaml by:
# 1. Parsing OCIRepository and HelmRelease to get chart URL and version
# 2. Running helm template with values from cm.yaml
# 3. Copying template files and merging mirror manifests with kustomization

# Activate devbox shell if available
if command -v devbox &> /dev/null; then
    eval "$(devbox shellenv)"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FLUX_DIR="$REPO_ROOT/applications/kommander-flux"
BOOTSTRAP_DIR="$FLUX_DIR/bootstrap"

# Find the latest version directory
LATEST_VERSION=$(find "$FLUX_DIR" -mindepth 1 -maxdepth 1 -type d ! -name bootstrap -printf "%f\n")
if [ -z "$LATEST_VERSION" ]; then
    echo "Error: Could not find a version directory in $FLUX_DIR"
    exit 1
fi

VERSION_DIR="$FLUX_DIR/$LATEST_VERSION"
HELMRELEASE_DIR="$VERSION_DIR/helmrelease"

echo "Using version directory: $LATEST_VERSION"

# Parse helmrelease.yaml to get chart URL and version
HELMRELEASE_FILE="$HELMRELEASE_DIR/helmrelease.yaml"
if [ ! -f "$HELMRELEASE_FILE" ]; then
    echo "Error: helmrelease.yaml not found at $HELMRELEASE_FILE"
    exit 1
fi

# Process the helmrelease.yaml to evaluate variables using envsubst
# Extract just the OCIRepository document (first document)
OCIRepo=$(yq eval 'select(.kind == "OCIRepository") | .' "$HELMRELEASE_FILE" 2>/dev/null)

# Set default values for variables
export releaseNamespace="kommander-flux"
export ociRegistryURL="oci://ghcr.io"

# Process the OCIRepository to evaluate variables
TEMP_PROCESSED=$(mktemp)
trap 'rm -f "$TEMP_VALUES" "$TEMP_PATCHES" "$TEMP_PROCESSED" 2>/dev/null' EXIT

echo "$OCIRepo" | envsubst > "$TEMP_PROCESSED"

# Extract OCI URL using yq
CHART_URL=$(yq eval '.spec.url' "$TEMP_PROCESSED" 2>/dev/null)
if [ -z "$CHART_URL" ] || [ "$CHART_URL" = "null" ]; then
    echo "Error: Could not extract URL from helmrelease.yaml"
    exit 1
fi

# Extract version/tag
VERSION=$(yq eval '.spec.ref.tag' "$TEMP_PROCESSED" 2>/dev/null)
if [ -z "$VERSION" ] || [ "$VERSION" = "null" ]; then
    echo "Error: Could not extract version/tag from helmrelease.yaml"
    exit 1
fi

echo "Chart URL: $CHART_URL"
echo "Version: $VERSION"

# Extract values from cm.yaml and write to temp file
CM_FILE="$HELMRELEASE_DIR/cm.yaml"
TEMP_VALUES=$(mktemp)

if [ ! -f "$CM_FILE" ]; then
    echo "Error: cm.yaml not found at $CM_FILE"
    exit 1
fi

# Extract the values.yaml content from the ConfigMap
yq eval '.data."values.yaml"' "$CM_FILE" > "$TEMP_VALUES"

if [ ! -s "$TEMP_VALUES" ]; then
    echo "Warning: Could not extract values from $CM_FILE"
    echo "" > "$TEMP_VALUES"
fi

# Ensure bootstrap directory exists
mkdir -p "$BOOTSTRAP_DIR"

# Run helm template
echo "Running helm template..."
helm template flux2 "$CHART_URL" \
  --version "$VERSION" \
  --namespace kommander-flux \
  --include-crds --no-hooks \
  -f "$TEMP_VALUES" \
  > "$BOOTSTRAP_DIR/flux-all.yaml"

echo "Generated flux-all.yaml"

# Update bootstrap kustomization.yaml
KUSTOMIZATION_FILE="$BOOTSTRAP_DIR/kustomization.yaml"

# Create or update kustomization.yaml with directory references
cat > "$KUSTOMIZATION_FILE" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - flux-all.yaml
  - ../${LATEST_VERSION}/templates
  - ../${LATEST_VERSION}/mirror
EOF

echo "Done! Generated flux-all.yaml and updated bootstrap kustomization."
