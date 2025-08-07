# KNative Image Management Scripts

This directory contains automation scripts for managing KNative Docker images, registry overrides for air-gapped deployments, and license entries in the Kommander Applications repository.

## Scripts Overview

### `extract-images.py`
Extracts Docker image references from KNative operator manifests and automatically generates registry overrides for air-gapped deployments. Features include:
- Multi-version support for eventing and serving components
- Automatic registry override generation with deployment/container name mapping
- Environment variable image detection and special formatting
- Reverse digest-to-tag lookup using Google Container Registry API
- Automatic cm.yaml configuration file updates

### `update-licenses.py`
Updates the `licenses.d2iq.yaml` file with KNative images extracted by the first script, ensuring proper license compliance.

---

## Prerequisites

### Python Environment Setup
```bash
# Create and activate virtual environment
python3 -m venv venv
source venv/bin/activate  # On macOS/Linux
# or
venv\Scripts\activate     # On Windows

# Install required dependencies
pip install docker-image-py
```

### Required Dependencies
- docker-image-py: For proper Docker image reference validation
- Standard library: subprocess, re, json, argparse, pathlib

---

## Usage

### Step 1: Extract KNative Images and Generate Registry Overrides

```bash
# Activate virtual environment
source venv/bin/activate

# Extract images and generate registry overrides for specific versions
python3 hack/knative/extract-images.py --eventing-version <eventing_version> --serving-version <serving_version> [--k-apps-version <k_apps_version>]

# Examples:
python3 hack/knative/extract-images.py --eventing-version 1.18.1 --serving-version 1.18.1
python3 hack/knative/extract-images.py --eventing-version 1.18.1 --serving-version 1.18.1 --k-apps-version 1.18.1
python3 hack/knative/extract-images.py --eventing-version 1.19.0 --serving-version 1.19.0
```

**What it does:**
- Fetches YAML manifests from knative/operator repository for both eventing and serving
- Extracts Docker image references using multiple advanced patterns including environment variables
- Validates image references using docker-image-py
- Performs reverse digest-to-tag lookup using Google Container Registry API v2
- Generates deployment/container name mappings for proper registry override format
- Automatically updates cm.yaml with properly formatted registry overrides
- Preserves existing config sections (deployment, istio, features, autoscaler)
- Saves extracted images to applications/knative/{version}/extra-images.txt

**Output:**
```
applications/knative/1.18.1/extra-images.txt                    # Raw image list
applications/knative/1.18.1/defaults/cm.yaml                   # Updated with registry overrides
```

**New Registry Override Features:**
- **Deployment/Container Format**: Uses proper `deployment-name/container-name` format instead of image paths
- **Environment Variable Handling**: Environment variable images use env var names as keys (e.g., `APISERVER_RA_IMAGE`)
- **No Duplicates**: Properly replaces existing registry sections without creating duplicates
- **Config Preservation**: Maintains critical config sections like `registriesSkippingTagResolving`
- **Comprehensive Mapping**: Covers both serving and eventing components with accurate deployment mappings

### Step 2: Update License File

```bash
# Update licenses with extracted images
python3 hack/knative/update-licenses.py <version>

# Examples:
python3 hack/knative/update-licenses.py 1.18.1
python3 hack/knative/update-licenses.py 1.19.0
```

**What it does:**
- Reads the extra-images.txt file generated in Step 1
- Completely replaces all existing KNative entries in licenses.d2iq.yaml
- Processes ALL images from extra-images.txt without deduplication
- Adds KNative operator images (not included in extra-images.txt)
- Uses proper version-specific ref format for all entries
- Maps images to correct GitHub repositories

**Output:**
- Updates licenses.d2iq.yaml with current image digests and version refs

---

## Script Details

### `extract-images.py`

#### Major Features:
- **Multi-version support**: Separate eventing and serving version specification
- **Advanced image extraction**: Multiple patterns for standard images, digests, and environment variables
- **Registry override generation**: Creates properly formatted registry overrides for air-gapped deployments
- **Digest-to-tag conversion**: Uses Google Container Registry API v2 to convert digest references to tagged versions
- **Automatic cm.yaml updates**: Updates configuration files with registry overrides while preserving existing config
- **Environment variable detection**: Special handling for images defined in environment variables
- **Docker validation**: Validates all extracted strings as proper Docker image references
- **GitHub API integration**: Fetches live manifests from the official KNative operator repository

#### Command Line Arguments:
```bash
--eventing-version  # Required: KNative eventing version (e.g., 1.18.1)
--serving-version   # Required: KNative serving version (e.g., 1.18.1)  
--k-apps-version    # Optional: Kommander apps version (defaults to serving_version)
```

#### Image Extraction Patterns:
1. **Standard image references**: `image: gcr.io/knative-releases/...`
2. **Digest references**: `gcr.io/knative-releases/...@sha256:...` (converted to tagged versions)
3. **Environment variable images**: Detected via pattern `name: IMAGE_NAME\nvalue: image-reference`
4. **ConfigMap references**: Images referenced in ConfigMaps or other resources

#### Registry Override Generation:
- **Deployment/Container Format**: Converts image paths to `deployment-name/container-name` format
- **Environment Variable Keys**: Environment variable images use env var names as keys
- **Comprehensive Mappings**: Covers both serving and eventing components with accurate deployment mappings
- **Config Preservation**: Maintains essential config sections like `registriesSkippingTagResolving`, `istio`, `features`

#### Example Registry Override Output:
```yaml
serving:
  manifest:
    spec:
      registry:
        override:
          # Pin serving images to specific tagged versions
          activator/activator: gcr.io/knative-releases/knative.dev/serving/cmd/activator:v1.18.1
          autoscaler/autoscaler: gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler:v1.18.1
          controller/controller: gcr.io/knative-releases/knative.dev/serving/cmd/controller:v1.18.1
      config:
        deployment:
          registriesSkippingTagResolving: "gcr.io"
        # ... other config preserved
```

#### Repository Structure:
```
cmd/operator/kodata/
  knative-eventing/
    {version}/
      200-eventing-core.yaml
      201-eventing-crds.yaml
      ...
  knative-serving/
    {version}/
      200-serving-core.yaml
      201-serving-crds.yaml
      ...
```

### `update-licenses.py`

#### Features:
- **Complete replacement**: Removes all existing KNative entries and rebuilds from scratch
- **No deduplication**: Processes every image in extra-images.txt exactly as listed
- **Version-aware**: Updates version references when new version is specified
- **Operator integration**: Automatically includes KNative operator images
- **Repository mapping**: Maps images to correct GitHub repositories
- **Clean insertion**: Places all KNative entries after Velero images in consistent order

#### Processing Logic:
1. **Read source**: Loads all images from extra-images.txt
2. **Add operators**: Includes operator images (not in extra-images.txt)
3. **Clean slate**: Removes ALL existing KNative entries from license file
4. **Rebuild**: Adds all images (operators first, then extra-images.txt content)
5. **Version update**: Uses provided version for ref formatting

#### Repository Mapping:
| Image Path | GitHub Repository |
|------------|------------------|
| knative.dev/eventing/* | https://github.com/knative/eventing |
| knative.dev/serving/* | https://github.com/knative/serving |
| knative.dev/pkg/* | https://github.com/knative/pkg |
| knative.dev/operator/* | https://github.com/knative/operator |
| aws-*, timer-source, log-sink, transform-jsonata | https://github.com/knative/eventing |

#### License Entry Format:
```yaml
# Operator images (use variable ref)
- container_image: gcr.io/knative-releases/knative.dev/operator/cmd/operator:v1.19.0
  sources:
    - license_path: LICENSE
      ref: knative-${image_tag}
      url: https://github.com/knative/operator

# Regular KNative images (use version-specific ref)
- container_image: gcr.io/knative-releases/image@sha256:...
  sources:
    - license_path: LICENSE
      ref: knative-v1.19.0
      url: https://github.com/knative/eventing
```

#### Image Processing:
- **Total images**: 30 (2 operator + 28 from extra-images.txt)
- **Operator images**: Always added first with `knative-${image_tag}` ref
- **Extra images**: Processed in order from extra-images.txt with `knative-v{version}` ref
- **Duplicates**: If extra-images.txt contains duplicates, all are included in license file

---

## Workflow Example

Complete workflow for updating KNative images to version 1.19.0:

```bash
# 1. Set up environment
source venv/bin/activate

# 2. Extract images and generate registry overrides
python3 hack/knative/extract-images.py --eventing-version 1.19.0 --serving-version 1.19.0
# Output: 
#   - applications/knative/1.19.0/extra-images.txt (extracted images)
#   - applications/knative/1.19.0/defaults/cm.yaml (updated with registry overrides)

# 3. Update license file
python3 hack/knative/update-licenses.py 1.19.0
# Output: Updated licenses.d2iq.yaml (30 total images: 2 operator + 28 extra)

# 4. Validate results (optional)
make validate-licenses
```

### Expected Output:
```
Extract Images Script:
  Extracting Docker images from KNative operator manifests
  Eventing version: 1.19.0
  Serving version: 1.19.0
  ======================================================================
  Found 28 images in eventing and serving manifests
  Generated registry overrides with deployment/container format
  Updated applications/knative/1.19.0/defaults/cm.yaml (preserved config sections)

Update Licenses Script:
  Found 28 images in extra-images.txt
  Removed 30 existing KNative entries
  Added 30 KNative entries after Velero
  Updated licenses.d2iq.yaml:
    - Operator images: 2
    - Extra images: 28
    - Used version: knative-v1.19.0
```

---

## File Structure

```
hack/knative/
  README.md                    # This documentation
  extract-images.py            # Image extraction script
  update-licenses.py           # License update script

applications/knative/
  {version}/
    extra-images.txt         # Generated image list

licenses.d2iq.yaml               # Updated license file
```

---

## Troubleshooting

### Registry Override Issues

**Problem**: Config sections are missing after running extract-images.py
**Solution**: The script now preserves all non-registry-override content in cm.yaml using line-by-line processing. Check that your cm.yaml has the expected config sections intact.

**Problem**: Registry overrides in wrong format
**Solution**: Ensure you're using the latest version of extract-images.py. The script now generates overrides in the correct deployment/container format:
```yaml
registry:
  override:
    eventing-controller/eventing-controller: my-registry.com/gcr.io/knative-releases/knative.dev/eventing/cmd/controller:v1.19.0
    eventing-webhook/eventing-webhook: my-registry.com/gcr.io/knative-releases/knative.dev/eventing/cmd/webhook:v1.19.0
```

**Problem**: Duplicate registry sections being created
**Solution**: The script now detects existing registry override sections and updates them in place instead of creating duplicates.

### Image Extraction Issues

**Problem**: Script fails with "Could not find tag for digest"
**Solution**: Some images may not have tags in the GCR API. The script will log these cases and continue processing other images.

**Problem**: Environment variable images not detected
**Solution**: Check that environment variables follow the pattern `*_IMAGE` in the YAML manifests. The script looks for this specific pattern and treats them specially.

**Problem**: GitHub API rate limiting
**Solution**: Set up a GitHub token in your environment to increase rate limits:
```bash
export GITHUB_TOKEN=your_token_here
```

### License Update Issues

**Problem**: Wrong number of images in licenses.d2iq.yaml
**Solution**: Verify that extra-images.txt exists and contains the expected number of images. The update-licenses.py script counts operator images (typically 2) plus extracted images.

**Problem**: Images not found in correct order
**Solution**: The script inserts KNative entries after Velero entries in the license file. Ensure Velero entries exist as landmarks.

### Common Issues

**Problem**: Virtual environment not activated
**Solution**: 
```bash
# Activate the virtual environment
source venv/bin/activate
```

**Problem**: Missing dependencies
**Solution**: Install the required dependencies:
```bash
pip install requests pyyaml
```

**Problem**: Invalid KNative version
**Error**: `applications/knative/1.99.0/extra-images.txt not found`
**Solution**: Check available versions at: https://github.com/knative/operator/releases

**Problem**: Network issues fetching manifests
**Error**: `Error fetching https://api.github.com/repos/knative/operator/contents/...`
**Solution**: 
- Check internet connectivity
- Verify GitHub API access

**Problem**: File not found errors
**Solution**: Run scripts from the repository root directory, not from the hack/knative/ directory.

**Problem**: License validation failures
**Solution**: 
```bash
# Run license validation to check for issues
make validate-licenses
```

### Debug Information:

Both scripts provide verbose output showing:
- Files being processed
- Images being extracted/updated
- Success/failure counts
- Final statistics

---

## Development Notes

### Script Architecture:
- **Simplified design**: Clean replacement strategy eliminates complex update logic
- **Error handling**: Graceful failure with informative messages
- **Validation**: Multiple layers of image reference validation
- **Logging**: Detailed progress reporting with counts and statistics
- **Source of truth**: Uses extra-images.txt as definitive image list

### Key Principles:
- **No deduplication**: Processes exactly what's in extra-images.txt
- **Clean replacement**: Remove all, then add all (no selective updates)
- **Version consistency**: All images use same version reference format
- **Predictable output**: Same input always produces same result

### Maintenance:
- Scripts are version-agnostic and should work with future KNative releases
- GitHub API integration uses public endpoints (no authentication required)
- Regular expression patterns may need updates if KNative changes manifest structure

### Testing:
```bash
# Test with known good version
python3 hack/knative/extract-images.py 1.18.1
python3 hack/knative/update-licenses.py 1.18.1

# Verify no errors and check counts
echo $?  # Should return 0

# Expected: 28 images in extra-images.txt + 2 operator images = 30 total
grep -c "gcr.io/knative-releases/" licenses.d2iq.yaml  # Should show 30

# Verify all images from extra-images.txt are included
wc -l applications/knative/1.18.1/extra-images.txt  # Should show 28
```

### Version Update Testing:
```bash
# Test version updating (1.18.1 → 1.19.0)
python3 hack/knative/update-licenses.py 1.19.0

# Verify version refs updated
grep -c "knative-v1.19.0" licenses.d2iq.yaml  # Should show 28
grep -c "knative-\${image_tag}" licenses.d2iq.yaml  # Should show 2 (operators)
```
