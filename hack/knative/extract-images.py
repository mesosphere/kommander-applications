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

def configmap_key_to_env_var(config_key):
    """Convert ConfigMap key to environment variable name format."""
    # Convert dashes to underscores and make uppercase
    # aws-ddb-streams-source -> AWS_DDB_STREAMS_SOURCE
    return config_key.replace('-', '_').upper()

def extract_configmap_images(yaml_content, default_version=None):
    """Extract image references from ConfigMap data sections only."""
    configmap_images = {}

    # Split into documents and process each one
    yaml_documents = re.split(r'\n---+\n', yaml_content)

    for yaml_doc in yaml_documents:
        if not yaml_doc.strip():
            continue

        lines = yaml_doc.split('\n')
        in_configmap = False
        in_data_section = False
        current_indent = 0

        for line in lines:
            stripped = line.strip()
            if not stripped or stripped.startswith('#'):
                continue

            # Check if this is a ConfigMap
            if stripped.startswith('kind:') and 'ConfigMap' in stripped:
                in_configmap = True
                continue

            # Skip if not in a ConfigMap
            if not in_configmap:
                continue

            # Check if we're entering the data section
            if stripped == 'data:':
                in_data_section = True
                current_indent = len(line) - len(line.lstrip())
                continue

            # Check if we're still in the data section
            if in_data_section:
                line_indent = len(line) - len(line.lstrip())

                # If indentation is less than or equal to data section, we've left it
                if line_indent <= current_indent and stripped:
                    in_data_section = False
                    continue

                # If we're in the data section, look for image references
                if ':' in stripped and 'gcr.io/' in stripped:
                    try:
                        key, value = stripped.split(':', 1)
                        key = key.strip()
                        value = value.strip()

                        if is_valid_docker_image(value):
                            # Do reverse lookup to find actual tag
                            actual_image = reverse_lookup_tag_from_digest(value, default_version)
                            configmap_images[key] = actual_image
                    except ValueError:
                        continue

    return configmap_images

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


def reverse_lookup_tag_from_digest(image_ref, default_tag=None):
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
            if default_tag:
                print(f"    Using default tag: v{default_tag}")
                return f"{base_image}:v{default_tag}"
            return image_ref

        try:
            tags_data = json.loads(content)
            if 'manifest' not in tags_data:
                print(f"    Warning: No manifest data found for {repo_path}")
                if default_tag:
                    print(f"    Using default tag: v{default_tag}")
                    return f"{base_image}:v{default_tag}"
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
            if default_tag:
                print(f"    Using default tag: v{default_tag}")
                return f"{base_image}:v{default_tag}"
            return image_ref

        except json.JSONDecodeError as e:
            print(f"    Warning: Error parsing tags response for {repo_path}: {e}")
            if default_tag:
                print(f"    Using default tag: v{default_tag}")
                return f"{base_image}:v{default_tag}"
            return image_ref

    except Exception as e:
        print(f"    Warning: Error in reverse lookup for {image_ref}: {e}")
        if default_tag:
            print(f"    Using default tag: v{default_tag}")
            # Parse the base image from the original reference
            base_image = image_ref.split('@sha256:')[0]
            return f"{base_image}:v{default_tag}"
        return image_ref

def extract_images_from_yaml(yaml_content, component_name, default_version=None):
    """Extract Docker image references from YAML content with component context and container/job mapping."""
    images = set()
    env_var_images = {}  # {env_var_name: image_reference}
    component_images = {}  # {image: {"component": "serving/eventing", "container_context": "deployment/container or job/container"}}
    env_only_images = set()  # Track images that only appear as environment variables

    # Pattern 1: Standard image references
    image_pattern = r'^\s*image:\s*["\']?([^"\'\s#]+)["\']?\s*(?:#.*)?$'

    # Pattern 2: Digest references (ConfigMaps, EnvVars, etc.)
    sha256_pattern = r'([a-zA-Z0-9.-]+(?:/[a-zA-Z0-9._-]+)*(?::[a-zA-Z0-9._-]+)?@sha256:[a-f0-9]{64})'

    # Pattern 3: Environment variable image references
    env_var_pattern = r'^\s*-\s*name:\s*([A-Z_]+_IMAGE)\s*\n\s*value:\s*([^\s#]+)'


    # Check for environment variable image patterns first (multi-line)
    env_matches = re.findall(env_var_pattern, yaml_content, re.MULTILINE)
    for env_name, image_ref in env_matches:
        if is_valid_docker_image(image_ref):
            # Do reverse lookup to find actual tag
            actual_image = reverse_lookup_tag_from_digest(image_ref, default_version)
            env_var_images[env_name] = actual_image
            env_only_images.add(actual_image)  # Track as env-only initially
            # Add to images set but do NOT add to component_images since it's env-only
            images.add(actual_image)
            print(f"    Found env var image: {env_name} -> {actual_image}")

    # Check for ConfigMap data image patterns (context-aware)
    configmap_images = extract_configmap_images(yaml_content, default_version)
    for config_key, actual_image in configmap_images.items():
        # Convert ConfigMap key to environment variable name
        env_var_name = configmap_key_to_env_var(config_key)
        env_var_images[env_var_name] = actual_image
        env_only_images.add(actual_image)  # Track as env-only
        # Add to images set but do NOT add to component_images since it's env-only
        images.add(actual_image)
        print(f"    Found ConfigMap image: {config_key} -> {env_var_name} -> {actual_image}")

    # Split YAML content by document separators to handle multiple manifests
    # Handle both single and consecutive document separators
    yaml_documents = re.split(r'\n---+\n', yaml_content)

    for doc_index, yaml_doc in enumerate(yaml_documents):
        if not yaml_doc.strip():
            continue

        # Extract deployment/job context from each YAML document
        lines = yaml_doc.split('\n')
        current_context = {"kind": None, "metadata_name": None, "generate_name": None, "container_name": None}

        i = 0
        while i < len(lines):
            line = lines[i]

            # Track current resource context
            if line.startswith('kind:'):
                current_context["kind"] = line.split(':', 1)[1].strip()
            elif line.strip().startswith('name:') and current_context["kind"]:
                # Only capture metadata name, not container names
                indent = len(line) - len(line.lstrip())
                if indent <= 4:  # Top-level metadata (allowing for some indentation)
                    current_context["metadata_name"] = line.split(':', 1)[1].strip()
            elif line.strip().startswith('generateName:') and current_context["kind"]:
                # Capture generateName for jobs
                indent = len(line) - len(line.lstrip())
                if indent <= 4:  # Top-level metadata
                    current_context["generate_name"] = line.split(':', 1)[1].strip()
            elif line.strip().startswith('- name:') and 'containers:' in yaml_doc[max(0, yaml_doc.find(line) - 200):yaml_doc.find(line)]:
                # Container name in containers section
                current_context["container_name"] = line.split(':', 1)[1].strip()

            # Check for standard image references
            match = re.match(image_pattern, line)
            if match:
                image = match.group(1)
                if is_valid_docker_image(image):
                    # Convert digest images to tagged versions, or keep tagged images as-is
                    if '@sha256:' in image:
                        final_image = reverse_lookup_tag_from_digest(image, default_version)
                    else:
                        final_image = image

                    # Add the final image (preferring tagged versions)
                    images.add(final_image)

                    # Remove from env-only set since it also appears as a container image
                    env_only_images.discard(final_image)

                    # Generate container context for the final image
                    container_context = generate_container_context(
                        final_image,
                        current_context,
                        component_name
                    )
                    component_images[final_image] = {
                        "component": component_name,
                        "container_context": container_context
                    }

            # Check for digest patterns
            sha256_matches = re.findall(sha256_pattern, line)
            for match in sha256_matches:
                if is_valid_docker_image(match):
                    # Convert digest to tagged version
                    tagged_image = reverse_lookup_tag_from_digest(match, default_version)

                    # Only add if we haven't already added this image
                    if tagged_image not in images:
                        images.add(tagged_image)

                        # Remove from env-only set since it also appears as a container image
                        env_only_images.discard(tagged_image)

                        # Generate container context for tagged image
                        container_context = generate_container_context(
                            tagged_image,
                            current_context,
                            component_name
                        )
                        component_images[tagged_image] = {
                            "component": component_name,
                            "container_context": container_context
                        }

            i += 1

    return sorted(images), env_var_images, component_images, env_only_images

def generate_container_context(image, current_context, component_name):
    """Generate the container context (deployment/container or job/container) for registry overrides."""
    # Extract image path for analysis
    if 'gcr.io/knative-releases/' in image:
        image_path = image.replace('gcr.io/knative-releases/', '').split(':')[0]
    else:
        image_path = image.split(':')[0]

    # Handle storage version migration and cleanup jobs specially
    if 'storageversion' in image_path or 'cleanup' in image_path:
        if current_context.get("kind") == "Job":
            metadata_name = current_context.get("metadata_name", "") or ""
            generate_name = current_context.get("generate_name", "") or ""
            container_name = current_context.get("container_name", "") or ""

            # Use generateName if available, otherwise fall back to metadata_name
            job_name_base = generate_name if generate_name else metadata_name

            print(f"      Processing job: kind={current_context.get('kind')}, name={metadata_name}, generateName={generate_name}, container={container_name}")
            print(f"      Job name base: {job_name_base}")
            print(f"      Image path: {image_path}")

            # Generate proper job-based override key based on generateName pattern
            if job_name_base:
                if "storage-version-migration-serving-" in job_name_base:
                    return f"storage-version-migration-serving-/{container_name}"
                elif "cleanup-serving-" in job_name_base:
                    return f"cleanup-serving-/{container_name}"
                elif "storage-version-migration-eventing-" in job_name_base:
                    return f"storage-version-migration-eventing-/{container_name}"
                elif "cleanup-eventing-" in job_name_base:
                    return f"cleanup-eventing-/{container_name}"
                elif "serving" in job_name_base:
                    if "cleanup" in image_path:
                        return f"cleanup-serving-/{container_name}"
                    else:
                        return f"storage-version-migration-serving-/{container_name}"
                elif "eventing" in job_name_base:
                    if "cleanup" in image_path:
                        return f"cleanup-eventing-/{container_name}"
                    else:
                        return f"storage-version-migration-eventing-/{container_name}"

            # Fallback based on component and image path
            if component_name == "knative-serving":
                if "cleanup" in image_path:
                    return "cleanup-serving-/cleanup"
                else:
                    return "storage-version-migration-serving-/migrate"
            else:  # knative-eventing
                if "cleanup" in image_path:
                    return "cleanup-eventing-/cleanup"
                else:
                    return "storage-version-migration-eventing-/migrate"

    # Handle standard deployments
    deployment_name = current_context.get("metadata_name", "") or ""
    container_name = current_context.get("container_name", "") or ""

    if deployment_name and container_name:
        return f"{deployment_name}/{container_name}"

    # Fallback to image-based mapping for known patterns
    return generate_fallback_context(image_path, component_name)

def generate_fallback_context(image_path, component_name):
    """Generate fallback container context based on image path analysis."""
    # Extract last component for unknown images
    image_name = image_path.split('/')[-1] if '/' in image_path else image_path
    return f"{image_name}/{image_name}"


def generate_registry_overrides(all_images, all_env_var_images, all_component_images, all_env_only_images, eventing_version, serving_version):
    """Generate registry override configuration for cm.yaml using component context."""
    serving_overrides = []
    eventing_overrides = []

    # Track override keys to avoid duplicates
    serving_keys = set()
    eventing_keys = set()

    # Process regular images using component context
    for image in sorted(all_images):
        if image not in all_component_images:
            continue  # Skip images without component context

        # Skip images that only appear as environment variables
        if image in all_env_only_images:
            continue  # This image should only be processed as an environment variable

        component_info = all_component_images[image]
        component = component_info["component"]
        container_context = component_info["container_context"]

        # Use the converted tagged image if available, otherwise use the original
        display_image = image

        # Special case for queue-proxy: use just "queue-proxy" instead of "queue-proxy/queue-proxy" or "queue/queue"
        if 'queue' in image and (container_context == "queue-proxy/queue-proxy" or container_context == "queue/queue"):
            container_context = "queue-proxy"

        # Create override entry
        override_entry = f"              {container_context}: {display_image}"

        if component == "knative-serving":
            if container_context not in serving_keys:
                serving_overrides.append(override_entry)
                serving_keys.add(container_context)
        elif component == "knative-eventing":
            if container_context not in eventing_keys:
                eventing_overrides.append(override_entry)
                eventing_keys.add(container_context)

    # Process environment variable images
    for env_name, image in sorted(all_env_var_images.items()):
        # Environment variable images use the env var name as the key
        # Determine component based on image path
        if 'knative.dev/serving' in image or ('knative.dev/pkg' in image and 'serving' in env_name.lower()):
            if env_name not in serving_keys:
                serving_overrides.append(f"              {env_name}: {image}")
                serving_keys.add(env_name)
        else:
            if env_name not in eventing_keys:
                eventing_overrides.append(f"              {env_name}: {image}")
                eventing_keys.add(env_name)

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

    if all_env_var_images:
        print()
        print("Note: Environment variable images found:")
        for env_name, image in sorted(all_env_var_images.items()):
            print(f"  {env_name} -> {image}")
        print("These use the environment variable name as the registry override key.")

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
        lines = content.split('\n')
        result_lines = []
        i = 0
        in_target_section = False
        in_registry_section = False
        target_section_indent = 0

        while i < len(lines):
            line = lines[i]

            # Check if we're entering the target section (serving or eventing)
            if re.match(rf'    {section_name}:', line):
                in_target_section = True
                target_section_indent = len(line) - len(line.lstrip())
                result_lines.append(line)
                i += 1
                continue

            # Check if we're leaving the target section (next section at same level)
            if in_target_section and line.strip() and len(line) - len(line.lstrip()) <= target_section_indent and not line.startswith('    #'):
                if not re.match(r'    [a-zA-Z]', line):
                    in_target_section = False
                elif re.match(r'    [a-zA-Z]', line) and not line.startswith(f'    {section_name}'):
                    in_target_section = False

            if in_target_section:
                # Look for registry section start
                if re.match(r'          registry:', line):
                    in_registry_section = True
                    result_lines.append(line)
                    i += 1

                    # Add override section header
                    if i < len(lines) and re.match(r'            override:', lines[i]):
                        result_lines.append(lines[i])
                        i += 1

                        # Skip existing override entries
                        while i < len(lines) and (re.match(r'              [#]', lines[i]) or re.match(r'              [a-zA-Z_]', lines[i])):
                            i += 1

                        # Add our overrides
                        result_lines.append(f'              # {comment}')
                        for override in overrides:
                            result_lines.append(override)

                        in_registry_section = False
                        continue
                    else:
                        # Add new override section
                        result_lines.append('            override:')
                        result_lines.append(f'              # {comment}')
                        for override in overrides:
                            result_lines.append(override)
                        in_registry_section = False
                        i += 1
                        continue

                # If we're not in a registry section, just copy the line
                if not in_registry_section:
                    result_lines.append(line)
            else:
                result_lines.append(line)

            i += 1

        return '\n'.join(result_lines)

    # Update serving section
    content = update_section_registry(content, "serving", serving_overrides, "Pin serving images to specific tagged versions")

    # Update eventing section
    content = update_section_registry(content, "eventing", eventing_overrides, "Pin eventing images to specific tagged versions")

    with open(cm_file, 'w') as f:
        f.write(content)

    print(f"Successfully updated {cm_file}")


def validate_version_exists(component, version):
    """Check if a specific version exists for a component in the knative/operator repository."""
    api_url = f"https://api.github.com/repos/knative/operator/contents/cmd/operator/kodata/{component}/{version}"
    
    content = run_curl(api_url)
    if not content:
        return False
        
    try:
        response = json.loads(content)
        # If we get a "message" field with "Not Found", the version doesn't exist
        if isinstance(response, dict) and 'message' in response and response['message'] == 'Not Found':
            return False
        # If we get a list, the version exists
        return isinstance(response, list)
    except Exception:
        return False

def get_available_versions(component):
    """Get list of available versions for a component."""
    api_url = f"https://api.github.com/repos/knative/operator/contents/cmd/operator/kodata/{component}"
    
    content = run_curl(api_url)
    if not content:
        return []
        
    try:
        folders = json.loads(content)
        versions = []
        for folder in folders:
            if folder['type'] == 'dir' and re.match(r'^\d+\.\d+\.\d+$', folder['name']):
                versions.append(folder['name'])
        return sorted(versions, key=lambda x: [int(i) for i in x.split('.')])
    except Exception:
        return []

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

def download_and_extract_images(files, component_name, default_version=None):
    """Download YAML files and extract images with component context."""
    all_images = set()
    all_env_var_images = {}
    all_component_images = {}
    all_env_only_images = set()

    print(f"\nProcessing {component_name} manifests...")

    for file_info in files:
        print(f"  Processing: {file_info['name']}")

        # Special debug for post-install jobs
        if "post-install" in file_info['name']:
            print(f"    DEBUG: Found post-install file: {file_info['name']}")

        yaml_content = run_curl(file_info['download_url'])
        if yaml_content:
            # Special debug for post-install jobs content
            if "post-install" in file_info['name']:
                print(f"    DEBUG: YAML content length: {len(yaml_content)}")
                yaml_documents = re.split(r'\n---+\n', yaml_content)
                print(f"    DEBUG: Found {len(yaml_documents)} documents")

            images, env_var_images, component_images, env_only_images = extract_images_from_yaml(yaml_content, component_name, default_version)
            all_images.update(images)
            all_env_var_images.update(env_var_images)
            all_component_images.update(component_images)
            all_env_only_images.update(env_only_images)

            if images:
                print(f"    Found {len(images)} images")
                for img in images:
                    context_info = component_images.get(img, {})
                    container_context = context_info.get("container_context", "unknown")
                    print(f"      {img} -> {container_context}")

            if env_var_images:
                print(f"    Found {len(env_var_images)} env var images")
                for env_name, img in env_var_images.items():
                    print(f"      {env_name} -> {img}")
        else:
            print(f"    Error downloading {file_info['name']}")

    return all_images, all_env_var_images, all_component_images, all_env_only_images

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

    # Validate that the specified versions exist
    print("Validating versions...")
    eventing_exists = validate_version_exists("knative-eventing", eventing_version)
    serving_exists = validate_version_exists("knative-serving", serving_version)
    
    errors = []
    warnings = []
    
    if not eventing_exists:
        available_eventing = get_available_versions("knative-eventing")
        latest_eventing = available_eventing[-1] if available_eventing else "unknown"
        errors.append(f"❌ KNative eventing version {eventing_version} does not exist!")
        errors.append(f"   Available eventing versions: {', '.join(available_eventing[-5:] if available_eventing else ['none'])}")
        errors.append(f"   Latest available: {latest_eventing}")
        
    if not serving_exists:
        available_serving = get_available_versions("knative-serving")
        latest_serving = available_serving[-1] if available_serving else "unknown"
        errors.append(f"❌ KNative serving version {serving_version} does not exist!")
        errors.append(f"   Available serving versions: {', '.join(available_serving[-5:] if available_serving else ['none'])}")
        errors.append(f"   Latest available: {latest_serving}")
    
    # Check for version mismatches (common issue)
    if eventing_exists and serving_exists:
        if eventing_version != serving_version:
            warnings.append(f"⚠️  Warning: Using different versions for eventing ({eventing_version}) and serving ({serving_version})")
            warnings.append(f"   This may cause compatibility issues.")
    
    # Print errors and warnings
    if errors:
        print("\nVersion Validation Errors:")
        print("-" * 40)
        for error in errors:
            print(error)
    
    if warnings:
        print("\nWarnings:")
        print("-" * 20)
        for warning in warnings:
            print(warning)
    
    # Exit if critical errors found
    if errors:
        print(f"\n❌ Cannot proceed with invalid versions. Please fix the version parameters and try again.")
        return 1
        
    if warnings:
        print(f"\n✅ Validation passed with warnings. Proceeding...\n")
    else:
        print(f"✅ Version validation passed. Proceeding...\n")

    all_images = set()
    all_env_var_images = {}
    all_component_images = {}
    all_env_only_images = set()

    # Process knative-eventing
    print("Fetching knative-eventing file list...")
    eventing_files = get_yaml_files_from_github_dir("knative-eventing", eventing_version)
    if eventing_files:
        print(f"Found {len(eventing_files)} eventing files")
        eventing_images, eventing_env_vars, eventing_component_images, eventing_env_only = download_and_extract_images(eventing_files, "knative-eventing", eventing_version)
        all_images.update(eventing_images)
        all_env_var_images.update(eventing_env_vars)
        all_component_images.update(eventing_component_images)
        all_env_only_images.update(eventing_env_only)
    else:
        print("No knative-eventing files found")

    # Process knative-serving
    print("\nFetching knative-serving file list...")
    serving_files = get_yaml_files_from_github_dir("knative-serving", serving_version)
    if serving_files:
        print(f"Found {len(serving_files)} serving files")
        serving_images, serving_env_vars, serving_component_images, serving_env_only = download_and_extract_images(serving_files, "knative-serving", serving_version)
        all_images.update(serving_images)
        all_env_var_images.update(serving_env_vars)
        all_component_images.update(serving_component_images)
        all_env_only_images.update(serving_env_only)
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
    serving_overrides, eventing_overrides = generate_registry_overrides(all_images, all_env_var_images, all_component_images, all_env_only_images, eventing_version, serving_version)

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
