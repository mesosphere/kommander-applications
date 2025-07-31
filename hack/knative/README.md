# KNative Image Management Scripts

This directory contains automation scripts for managing KNative Docker images and their license entries in the Kommander Applications repository.

## Scripts Overview

### `extract-images.py`
Extracts Docker image references from KNative operator manifests by fetching YAML files from the official KNative operator repository.

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

### Step 1: Extract KNative Images

```bash
# Activate virtual environment
source venv/bin/activate

# Extract images for a specific KNative version
python3 hack/knative/extract-images.py <version>

# Examples:
python3 hack/knative/extract-images.py 1.18.1
python3 hack/knative/extract-images.py 1.19.0
```

**What it does:**
- Fetches YAML manifests from knative/operator repository
- Scans both knative-eventing and knative-serving components
- Extracts Docker image references using multiple regex patterns
- Validates image references using docker-image-py
- Saves results to applications/knative/{version}/extra-images.txt

**Output:**
```
applications/knative/1.18.1/extra-images.txt
```

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

**Key Features:**
- **Clean replacement**: Removes all existing KNative entries and adds fresh ones
- **No deduplication**: Processes every image in extra-images.txt, including duplicates
- **Source of truth**: Uses extra-images.txt + version parameter as definitive source
- **Operator image management**: Automatically includes operator images with correct versioning
- **Positioning**: Places operator images first, followed by extra-images.txt content

**Output:**
- Updates licenses.d2iq.yaml with current image digests and version refs

---

## Script Details

### `extract-images.py`

#### Features:
- Multi-pattern extraction: Uses both standard image: patterns and @sha256 digest patterns
- Comprehensive scanning: Covers ConfigMaps, EnvVars, and standard Kubernetes manifests
- Docker validation: Validates all extracted strings as proper Docker image references
- GitHub API integration: Fetches live manifests from the official KNative operator repository

#### Image Extraction Patterns:
1. Standard image references: image: gcr.io/knative-releases/...
2. Digest references: gcr.io/knative-releases/...@sha256:...
3. Embedded references: Images referenced in ConfigMaps or environment variables

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

# 2. Extract images from KNative operator manifests
python3 hack/knative/extract-images.py 1.19.0
# Output: applications/knative/1.19.0/extra-images.txt (28 images)

# 3. Update license file
python3 hack/knative/update-licenses.py 1.19.0
# Output: Updated licenses.d2iq.yaml (30 total images: 2 operator + 28 extra)

# 4. Validate results (optional)
make validate-licenses
```

### Expected Output:
```
Found 28 images in extra-images.txt
Removed 30 existing KNative entries
Added 30 KNative entries after Velero

Updated licenses.d2iq.yaml:
  - Removed 30 existing KNative entries
  - Added 30 new KNative entries
  - Used version: knative-v1.19.0
  - Operator images: 2
  - Extra images: 28
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

### Common Issues:

1. Virtual environment not activated
   ```bash
   # Solution: Activate the virtual environment
   source venv/bin/activate
   ```

2. Missing docker-image-py dependency
   ```bash
   # Solution: Install the required dependency
   pip install docker-image-py
   ```

3. Invalid KNative version
   ```
   Error: applications/knative/1.99.0/extra-images.txt not found
   ```
   - Check available versions at: https://github.com/knative/operator/releases

4. Network issues fetching manifests
   ```
   Error fetching https://api.github.com/repos/knative/operator/contents/...
   ```
   - Check internet connectivity
   - Verify GitHub API access

5. License validation failures
   ```bash
   # Run license validation to check for issues
   make validate-licenses
   ```

6. Missing images in license file
   ```bash
   # Check if all images from extra-images.txt are included
   wc -l applications/knative/1.19.0/extra-images.txt  # Should match extra images count
   grep -c "gcr.io/knative-releases/" licenses.d2iq.yaml  # Should be extra + 2 operators
   ```

7. Wrong version references
   ```bash
   # Check version consistency
   grep "knative-v" licenses.d2iq.yaml | head -5  # Should show current version
   grep "knative-\${image_tag}" licenses.d2iq.yaml  # Should show exactly 2 (operators)
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
