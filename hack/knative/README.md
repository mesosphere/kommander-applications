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
- Updates existing KNative entries in licenses.d2iq.yaml
- Adds new KNative images if they don't exist
- Uses proper knative-v{version} ref format
- Maps images to correct GitHub repositories

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
├── knative-eventing/
│   └── {version}/
│       ├── 200-eventing-core.yaml
│       ├── 201-eventing-crds.yaml
│       └── ...
└── knative-serving/
    └── {version}/
        ├── 200-serving-core.yaml
        ├── 201-serving-crds.yaml
        └── ...
```

### `update-licenses.py`

#### Features:
- Smart updates: Updates existing entries rather than duplicating
- Version-aware: Uses the specified version for proper ref formatting
- Repository mapping: Maps images to correct GitHub repositories
- Insertion logic: Adds new images after existing KNative entries

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
- container_image: gcr.io/knative-releases/image@sha256:...
  sources:
    - license_path: LICENSE
      ref: knative-v1.18.1
      url: https://github.com/knative/eventing
```

---

## Workflow Example

Complete workflow for updating KNative images to version 1.19.0:

```bash
# 1. Set up environment
source venv/bin/activate

# 2. Extract images from KNative operator manifests
python3 hack/knative/extract-images.py 1.19.0
# Output: applications/knative/1.19.0/extra-images.txt

# 3. Update license file
python3 hack/knative/update-licenses.py 1.19.0
# Output: Updated licenses.d2iq.yaml

# 4. Validate results (optional)
make validate-licenses
```

---

## File Structure

```
hack/knative/
├── README.md                    # This documentation
├── extract-images.py            # Image extraction script
└── update-licenses.py           # License update script

applications/knative/
└── {version}/
    └── extra-images.txt         # Generated image list

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

### Debug Information:

Both scripts provide verbose output showing:
- Files being processed
- Images being extracted/updated
- Success/failure counts
- Final statistics

---

## Development Notes

### Script Architecture:
- Modular design: Clear separation of concerns
- Error handling: Graceful failure with informative messages
- Validation: Multiple layers of image reference validation
- Logging: Detailed progress reporting

### Maintenance:
- Scripts are version-agnostic and should work with future KNative releases
- GitHub API integration uses public endpoints (no authentication required)
- Regular expression patterns may need updates if KNative changes manifest structure

### Testing:
```bash
# Test with known good version
python3 hack/knative/extract-images.py 1.18.1
python3 hack/knative/update-licenses.py 1.18.1

# Verify no errors
echo $?  # Should return 0
```
