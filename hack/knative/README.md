# KNative Image Management Scripts

Two Python scripts for managing KNative Docker images in Kommander Applications:

1. `extract-images.py` - Extracts images from KNative operator manifests and generates registry overrides
2. `update-licenses.py` - Updates license file with extracted KNative images

## Prerequisites

```bash
# Install required dependency
pip install docker-image-py
```

## extract-images.py

Downloads KNative operator manifests from GitHub, extracts Docker image references, and automatically updates the cm.yaml configuration file with registry overrides for air-gapped deployments.

### Usage

```bash
python3 hack/knative/extract-images.py --eventing-version <version> --serving-version <version> [--k-apps-version <version>]
```

### Options

- `--eventing-version` (required): KNative eventing version (e.g., 1.18.1)
- `--serving-version` (required): KNative serving version (e.g., 1.18.1)
- `--k-apps-version` (optional): Output directory version (defaults to serving version)

### Version Management

**Important**: The script validates that both eventing and serving versions exist in [the knative/operator repository](https://github.com/knative/operator/tree/main/cmd/operator/kodata) before proceeding. If a version doesn't exist, it will show available versions and exit with an error.

**Directory Structure**: The script creates/updates files in `applications/knative/{k-apps-version}/`. If you're upgrading from an existing version:

1. **Rename the existing directory** to your target version first (e.g., `mv applications/knative/1.18.1 applications/knative/1.19.6`)
2. **Run the script** with `--k-apps-version` matching your renamed directory
3. The script will update `cm.yaml` and regenerate `extra-images.txt` in the target directory

**Version Compatibility**: Different eventing and serving versions can be used, but the script will warn about potential compatibility issues.

### What it does

1. Fetches YAML manifests from knative/operator GitHub repository
2. Extracts Docker image references using regex patterns
3. Converts digest references to tagged versions using Google Container Registry API
4. Maps images to deployment/container names for registry overrides
5. Updates applications/knative/{version}/defaults/cm.yaml with registry overrides
6. Saves all images to applications/knative/{version}/extra-images.txt

### Key features

- Handles environment variable images (APISERVER_RA_IMAGE, DISPATCHER_IMAGE)
- Converts SHA digest references to version tags
- Prevents duplicate entries
- Preserves existing configuration sections
- Special handling for queue-proxy container naming

### Output files

- `applications/knative/{version}/extra-images.txt` - List of all extracted images
- `applications/knative/{version}/defaults/cm.yaml` - Updated with registry overrides

## update-licenses.py

Updates the licenses.d2iq.yaml file with KNative images from the extracted image list.

### Usage

```bash
python3 hack/knative/update-licenses.py <version>
```

### Options

- `version` (required): KNative version (e.g., 1.18.1)

### What it does

1. Reads images from applications/knative/{version}/extra-images.txt
2. Adds KNative operator images (not in extra-images.txt)
3. Removes all existing KNative entries from licenses.d2iq.yaml
4. Adds all images with proper license information and GitHub repository URLs
5. Uses version-specific refs (knative-v{version} for regular images, knative-${image_tag} for operators)

### Repository mapping

- knative.dev/eventing/* -> https://github.com/knative/eventing
- knative.dev/serving/* -> https://github.com/knative/serving
- knative.dev/pkg/* -> https://github.com/knative/pkg
- knative.dev/operator/* -> https://github.com/knative/operator
- aws-*, timer-source, log-sink, transform-jsonata -> https://github.com/knative/eventing

## Complete workflow

```bash
# 1. Rename existing directory (if upgrading)
mv applications/knative/1.18.1 applications/knative/1.19.6

# 2. Extract images and generate registry overrides (use latest available versions)
python3 hack/knative/extract-images.py --eventing-version 1.19.5 --serving-version 1.19.6 --k-apps-version 1.19.6

# 3. Update license file
python3 hack/knative/update-licenses.py 1.19.6
```

### Example: Version mismatch handling
```bash
# This will fail with clear error message:
python3 hack/knative/extract-images.py --eventing-version 1.19.6 --serving-version 1.19.6

# This will work with warning about version mismatch:
python3 hack/knative/extract-images.py --eventing-version 1.19.5 --serving-version 1.19.6 --k-apps-version 1.19.6
```

## Troubleshooting

**Script fails with "Could not find tag for digest"**
- Script automatically generates fallback tags and continues

**Registry overrides in wrong format**
- Update to latest script version for proper deployment/container format

**Duplicate images**
- Script now deduplicates automatically

**Environment variable images not detected**
- Check YAML manifests use *_IMAGE pattern for environment variables

**License validation failures**
- Run `make validate-licenses` to check for issues
