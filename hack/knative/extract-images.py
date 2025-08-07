#!/usr/bin/env python3
"""
Extract Docker images from KNative operator manifests.

Requires the 'docker-image-py' library for proper Docker image reference validation:
    pip install docker-image-py

Usage:
    python3 hack/knative/extract-images.py --eventing-version <version> --serving-version <version> [--k-apps-version <version>]
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


def reverse_lookup_tag_from_digest(image_ref):
    """Reverse lookup to find the actual tag for a digest-based image reference."""
    if '@sha256:' not in image_ref:
        return image_ref  # Already a tagged image
    
    # Check if this is a tagged image with digest (tag:version@sha256:...)
    if ':v' in image_ref and '@sha256:' in image_ref:
        # Extract just the tagged part (remove the digest)
        tagged_part = image_ref.split('@sha256:')[0]
        print(f"    Image already tagged, removing digest: {tagged_part}")
        return tagged_part
    
    try:
        # Parse the image reference
        base_image, digest = image_ref.split('@sha256:', 1)
        digest = f'sha256:{digest}'
        
        # Extract repository path from gcr.io/knative-releases/image:tag format
        if 'gcr.io/knative-releases/' not in base_image:
            return image_ref  # Not a knative image
            
        repo_path = base_image.replace('gcr.io/knative-releases/', '')
        
        # Query GCR API to get all tags for this repository
        api_url = f"https://gcr.io/v2/knative-releases/{repo_path}/tags/list"
        
        content = run_curl(api_url)
        if not content:
            print(f"    Warning: Could not fetch tags for {repo_path}")
            return image_ref
            
        try:
            tags_data = json.loads(content)
            if 'manifest' not in tags_data:
                print(f"    Warning: No manifest data found for {repo_path}")
                return image_ref
                
            # Check if our digest exists in the manifest mapping
            if digest in tags_data['manifest']:
                manifest_info = tags_data['manifest'][digest]
                if 'tag' in manifest_info and manifest_info['tag']:
                    # Find the first version tag (starts with 'v')
                    for tag in manifest_info['tag']:
                        if tag.startswith('v'):
                            print(f"    Found tag {tag} for digest {digest[:12]}...")
                            return f"{base_image}:{tag}"
                    
                    # If no version tag found, use the first available tag
                    tag = manifest_info['tag'][0]
                    print(f"    Found tag {tag} for digest {digest[:12]}...")
                    return f"{base_image}:{tag}"
                        
            print(f"    Warning: No matching tag found for digest {digest[:12]}...")
            return image_ref
            
        except json.JSONDecodeError as e:
            print(f"    Warning: Error parsing tags response for {repo_path}: {e}")
            return image_ref
            
    except Exception as e:
        print(f"    Warning: Error in reverse lookup for {image_ref}: {e}")
        return image_ref

def extract_images_from_yaml(yaml_content):
    """Extract Docker image references from YAML content and do reverse lookup for actual tags."""
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
                # Do reverse lookup to find actual tag
                actual_image = reverse_lookup_tag_from_digest(image)
                images.add(actual_image)

        # Check for digest patterns
        sha256_matches = re.findall(sha256_pattern, line)
        for match in sha256_matches:
            if is_valid_docker_image(match):
                # Do reverse lookup to find actual tag
                actual_image = reverse_lookup_tag_from_digest(match)
                images.add(actual_image)

    return sorted(images)


def generate_registry_overrides(all_images, eventing_version, serving_version):
    """Generate registry override configuration for cm.yaml."""
    serving_overrides = []
    eventing_overrides = []
    
    for image in sorted(all_images):
        if '@sha256:' in image:
            continue  # Skip digest-based images that weren't converted
            
        # Extract the image path after gcr.io/knative-releases/
        if 'gcr.io/knative-releases/' not in image:
            continue
            
        image_path = image.replace('gcr.io/knative-releases/', '')
        
        # Split into path and tag
        if ':' in image_path:
            path, tag = image_path.rsplit(':', 1)
        else:
            continue
            
        # Determine if this is a serving or eventing image
        if 'knative.dev/serving' in path:
            serving_overrides.append(f"              {path}: {image}")
        elif 'knative.dev/eventing' in path:
            eventing_overrides.append(f"              {path}: {image}")
        elif 'knative.dev/pkg' in path:
            # pkg images are used by serving
            serving_overrides.append(f"              {path}: {image}")
        else:
            # Standalone images like aws-*, log-sink, timer-source, transform-jsonata
            # These are typically eventing-related
            image_name = path.split('/')[-1] if '/' in path else path
            eventing_overrides.append(f"              {image_name}: {image}")
    
    print("\n" + "="*70)
    print("REGISTRY OVERRIDE CONFIGURATION")
    print("="*70)
    print("Add this to applications/knative/{version}/defaults/cm.yaml:")
    print()
    
    print("For serving section:")
    print("          registry:")
    print("            override:")
    print("              # Pin serving images to specific tagged versions")
    for override in serving_overrides:
        print(override)
    
    print()
    print("For eventing section:")
    print("          registry:")
    print("            override:")
    print("              # Pin eventing images to specific tagged versions")
    for override in eventing_overrides:
        print(override)
    
    return serving_overrides, eventing_overrides


def update_cm_yaml(k_apps_version, serving_overrides, eventing_overrides):
    """Update the cm.yaml file with registry overrides."""
    script_dir = Path(__file__).parent
    repo_root = script_dir.parent.parent
    cm_file = repo_root / "applications" / "knative" / k_apps_version / "defaults" / "cm.yaml"
    
    if not cm_file.exists():
        print(f"Warning: {cm_file} does not exist, skipping automatic update")
        return
    
    print(f"\nUpdating {cm_file} with registry overrides...")
    
    with open(cm_file, 'r') as f:
        content = f.read()
    
    # Helper function to update registry overrides for a section
    def update_section_registry(content, section_name, overrides, comment):
        # Build replacement text for overrides
        overrides_text = f'          registry:\n            override:\n              # {comment}\n'
        for override in overrides:
            overrides_text += override + '\n'
        
        # Find the section boundary - look for the next top-level section or end of content
        section_start_pattern = rf'(    {section_name}:)'
        section_match = re.search(section_start_pattern, content)
        if not section_match:
            print(f"Warning: Could not find {section_name} section")
            return content
        
        section_start = section_match.start()
        
        # Find the end of this section (next section starting with 4 spaces or end of content)
        remaining_content = content[section_start:]
        next_section_pattern = r'\n    [a-zA-Z]'
        next_section_match = re.search(next_section_pattern, remaining_content[20:])  # Skip first 20 chars to avoid matching our own section
        
        if next_section_match:
            section_end = section_start + 20 + next_section_match.start() + 1  # +1 to include the newline
        else:
            section_end = len(content)
        
        section_content = content[section_start:section_end]
        
        # Check if registry section already exists in this section
        existing_registry_pattern = r'(.*?          version: "[^"]+").*?          registry:\s*\n            override:\s*\n((?:              .*\n)*)'
        existing_match = re.search(existing_registry_pattern, section_content, flags=re.DOTALL)
        
        if existing_match:
            # Replace existing registry section
            new_section_content = re.sub(existing_registry_pattern, rf'\1\n{overrides_text}', section_content, flags=re.DOTALL)
        else:
            # Add new registry section after version line
            version_pattern = r'(          version: "[^"]+")'
            new_section_content = re.sub(version_pattern, rf'\1\n{overrides_text.rstrip()}', section_content)
        
        # Replace the section in the full content
        new_content = content[:section_start] + new_section_content + content[section_end:]
        return new_content
    
    # Update serving section
    content = update_section_registry(content, "serving", serving_overrides, "Pin serving images to specific tagged versions")
    
    # Update eventing section
    content = update_section_registry(content, "eventing", eventing_overrides, "Pin eventing images to specific tagged versions")
    
    with open(cm_file, 'w') as f:
        f.write(content)
    
    print(f"Successfully updated {cm_file}")


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
    parser.add_argument('--eventing-version', required=True, help='KNative eventing version (e.g., 1.18.1)')
    parser.add_argument('--serving-version', required=True, help='KNative serving version (e.g., 1.18.1)')
    parser.add_argument('--k-apps-version', help='Kommander applications version for output directory (defaults to serving_version)')
    args = parser.parse_args()

    eventing_version = args.eventing_version
    serving_version = args.serving_version
    k_apps_version = getattr(args, 'k_apps_version') or serving_version

    print(f"Extracting Docker images from KNative operator manifests")
    print(f"Eventing version: {eventing_version}")
    print(f"Serving version: {serving_version}")
    print(f"K-apps version: {k_apps_version}")
    print("=" * 70)

    all_images = set()

    # Process knative-eventing
    print("Fetching knative-eventing file list...")
    eventing_files = get_yaml_files_from_github_dir("knative-eventing", eventing_version)
    if eventing_files:
        print(f"Found {len(eventing_files)} eventing files")
        eventing_images = download_and_extract_images(eventing_files, "knative-eventing")
        all_images.update(eventing_images)
    else:
        print("No knative-eventing files found")

    # Process knative-serving
    print("\nFetching knative-serving file list...")
    serving_files = get_yaml_files_from_github_dir("knative-serving", serving_version)
    if serving_files:
        print(f"Found {len(serving_files)} serving files")
        serving_images = download_and_extract_images(serving_files, "knative-serving")
        all_images.update(serving_images)
    else:
        print("No knative-serving files found")

    # Write results to file - use Path for proper directory resolution
    script_dir = Path(__file__).parent
    repo_root = script_dir.parent.parent
    knative_dir = repo_root / "applications" / "knative" / k_apps_version
    knative_dir.mkdir(parents=True, exist_ok=True)

    output_file = knative_dir / "extra-images.txt"

    with open(output_file, 'w') as f:
        for image in sorted(all_images):
            f.write(f"{image}\n")

    # Generate registry overrides configuration
    serving_overrides, eventing_overrides = generate_registry_overrides(all_images, eventing_version, serving_version)
    
    # Optionally update cm.yaml automatically
    try:
        update_cm_yaml(k_apps_version, serving_overrides, eventing_overrides)
    except Exception as e:
        print(f"Warning: Could not automatically update cm.yaml: {e}")

    print(f"\nResults:")
    print("=" * 20)
    print(f"Total unique images found: {len(all_images)}")
    print(f"Output written to: {output_file}")
    print(f"Generated on: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"Source: knative/operator repository (eventing: v{eventing_version}, serving: v{serving_version})")

    if all_images:
        print(f"\nAll extracted images:")
        print("-" * 30)
        for image in sorted(all_images):
            print(image)
    else:
        print("\nNo images found. Please check the versions and try again.")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
