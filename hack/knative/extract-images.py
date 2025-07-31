#!/usr/bin/env python3
"""
Extract Docker images from KNative operator manifests.

Requires the 'docker-image-py' library for proper Docker image reference validation:
    pip install docker-image-py

Usage:
    python3 hack/knative/extract-images.py <version>
"""

import argparse
import subprocess
import re
import sys
import json
import os
from datetime import datetime
from pathlib import Path
from docker_image import reference

def is_valid_docker_image(image_str):
    """Validate if a string is a valid Docker image reference using docker-image-py."""
    if not image_str or not isinstance(image_str, str):
        return False

    try:
        ref = reference.Reference.parse(image_str.strip())

        # Reject obvious version strings that aren't real image names
        image_name = str(ref).split('@')[0].split(':')[0]
        if '/' in image_name:
            image_name = image_name.split('/')[-1]

        if re.match(r'^v?\d+\.\d+(\.\d+)?$', image_name):
            return False

        return True
    except Exception:
        return False

def run_curl(url):
    """Run curl command and return output."""
    try:
        result = subprocess.run(['curl', '-sL', url], capture_output=True, text=True)
        if result.returncode == 0:
            return result.stdout
        else:
            print(f"Error fetching {url}: {result.stderr}")
            return None
    except Exception as e:
        print(f"Error running curl for {url}: {e}")
        return None

def extract_images_from_yaml(yaml_content):
    """Extract Docker image references from YAML content."""
    images = set()

    # Pattern 1: Standard image references
    image_pattern = r'^\s*image:\s*["\']?([^"\'\s#]+)["\']?\s*(?:#.*)?$'

    # Pattern 2: Digest references (ConfigMaps, EnvVars, etc.)
    sha256_pattern = r'([a-zA-Z0-9.-]+(?:/[a-zA-Z0-9._-]+)*(?::[a-zA-Z0-9._-]+)?@sha256:[a-f0-9]{64})'

    for line in yaml_content.split('\n'):
        # Check for standard image references
        match = re.match(image_pattern, line)
        if match:
            image = match.group(1)
            if is_valid_docker_image(image):
                images.add(image)

        # Check for digest patterns
        sha256_matches = re.findall(sha256_pattern, line)
        for match in sha256_matches:
            if is_valid_docker_image(match):
                images.add(match)

    return sorted(images)

def get_yaml_files_from_github_dir(repo_path, version):
    """Get list of YAML files from a GitHub directory."""
    api_url = f"https://api.github.com/repos/knative/operator/contents/cmd/operator/kodata/{repo_path}/{version}"

    content = run_curl(api_url)
    if not content:
        return []

    try:
        files = json.loads(content)
        yaml_files = []

        for file_info in files:
            if file_info['name'].endswith(('.yaml', '.yml')):
                yaml_files.append({
                    'name': file_info['name'],
                    'download_url': file_info['download_url']
                })

        return yaml_files
    except Exception as e:
        print(f"Error parsing JSON from {api_url}: {e}")
        return []

def download_and_extract_images(files, component_name):
    """Download YAML files and extract images."""
    all_images = set()

    print(f"\nProcessing {component_name} manifests...")

    for file_info in files:
        print(f"  Processing: {file_info['name']}")

        yaml_content = run_curl(file_info['download_url'])
        if yaml_content:
            images = extract_images_from_yaml(yaml_content)
            all_images.update(images)

            if images:
                print(f"    Found {len(images)} images")
                for img in images:
                    print(f"      {img}")
        else:
            print(f"    Error downloading {file_info['name']}")

    return all_images

def main():
    parser = argparse.ArgumentParser(description='Extract Docker images from KNative operator manifests')
    parser.add_argument('version', help='KNative version (e.g., 1.18.1, 1.19.0)')
    args = parser.parse_args()

    version = args.version

    print(f"Extracting Docker images from KNative operator manifests for version {version}")
    print("=" * 70)

    all_images = set()

    # Process knative-eventing
    print("Fetching knative-eventing file list...")
    eventing_files = get_yaml_files_from_github_dir("knative-eventing", version)
    if eventing_files:
        print(f"Found {len(eventing_files)} eventing files")
        eventing_images = download_and_extract_images(eventing_files, "knative-eventing")
        all_images.update(eventing_images)
    else:
        print("No knative-eventing files found")

    # Process knative-serving
    print("\nFetching knative-serving file list...")
    serving_files = get_yaml_files_from_github_dir("knative-serving", version)
    if serving_files:
        print(f"Found {len(serving_files)} serving files")
        serving_images = download_and_extract_images(serving_files, "knative-serving")
        all_images.update(serving_images)
    else:
        print("No knative-serving files found")

    # Write results to file - use Path for proper directory resolution
    script_dir = Path(__file__).parent
    repo_root = script_dir.parent.parent
    knative_dir = repo_root / "applications" / "knative" / version
    knative_dir.mkdir(parents=True, exist_ok=True)

    output_file = knative_dir / "extra-images.txt"

    with open(output_file, 'w') as f:
        for image in sorted(all_images):
            f.write(f"{image}\n")

    print(f"\nResults:")
    print("=" * 20)
    print(f"Total unique images found: {len(all_images)}")
    print(f"Output written to: {output_file}")
    print(f"Generated on: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"Source: knative/operator repository v{version}")

    if all_images:
        print(f"\nAll extracted images:")
        print("-" * 30)
        for image in sorted(all_images):
            print(image)
    else:
        print("\nNo images found. Please check the version and try again.")
        return 1

    return 0

if __name__ == "__main__":
    sys.exit(main())
