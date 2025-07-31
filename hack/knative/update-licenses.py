#!/usr/bin/env python3
"""
Update licenses.d2iq.yaml with KNative images from extra-images.txt

Usage:
    python3 hack/knative/update-licenses.py <version>
"""

import argparse
import re
import sys
from pathlib import Path


def parse_image_reference(image_ref):
    """Parse a Docker image reference into components."""
    if '@sha256:' in image_ref:
        image_part, digest = image_ref.split('@sha256:', 1)
        digest = f'sha256:{digest}'
    else:
        image_part = image_ref
        digest = None

    if ':' in image_part and not image_part.count(':') > 1:
        parts = image_part.split('/')
        if len(parts) > 1 and ':' in parts[0] and parts[0].split(':')[1].isdigit():
            # Registry:port format
            tag = None
        else:
            # Image:tag format
            image_part, tag = image_part.rsplit(':', 1)
    else:
        tag = None

    if '/' in image_part:
        parts = image_part.split('/', 1)
        if '.' in parts[0] or ':' in parts[0]:
            registry, repository = parts[0], parts[1]
        else:
            registry, repository = 'docker.io', image_part
    else:
        registry, repository = 'docker.io', f'library/{image_part}'

    return registry, repository, tag, digest


def get_repository_url(image_ref):
    """Determine the appropriate GitHub repository URL for a KNative image."""
    _, repository, _, _ = parse_image_reference(image_ref)

    if 'knative.dev/eventing' in repository:
        return 'https://github.com/knative/eventing'
    elif 'knative.dev/serving' in repository:
        return 'https://github.com/knative/serving'
    elif 'knative.dev/pkg' in repository:
        return 'https://github.com/knative/pkg'
    elif 'knative.dev/operator' in repository:
        return 'https://github.com/knative/operator'
    else:
        # For standalone images like aws-*, timer-source, log-sink, transform-jsonata
        return 'https://github.com/knative/eventing'


def extract_base_image_name(image_ref):
    """Extract the base image name without tag/digest for comparison."""
    if '@sha256:' in image_ref:
        base = image_ref.split('@sha256:')[0]
        # Handle special case like transform-jsonata:v1.19.0@sha256:...
        if ':' in base and not base.endswith(':latest'):
            return base.rsplit(':', 1)[0]
        return base
    elif ':' in image_ref and not image_ref.endswith(':latest'):
        return image_ref.rsplit(':', 1)[0]
    else:
        return image_ref


def create_license_entry_text(image_ref, version):
    """Create the text for a license entry."""
    repo_url = get_repository_url(image_ref)
    return (f"  - container_image: {image_ref}\n"
            f"    sources:\n"
            f"      - license_path: LICENSE\n"
            f"        ref: knative-v{version}\n"
            f"        url: {repo_url}\n")


def main():
    parser = argparse.ArgumentParser(description='Update licenses.d2iq.yaml with KNative images')
    parser.add_argument('version', help='KNative version (e.g., 1.19.0)')
    args = parser.parse_args()

    version = args.version

    # Read the extra images
    extra_images_file = Path(f"applications/knative/{version}/extra-images.txt")
    if not extra_images_file.exists():
        print(f"Error: {extra_images_file} not found")
        return 1

    with open(extra_images_file) as f:
        extra_images = [line.strip() for line in f if line.strip()]

    print(f"Found {len(extra_images)} images in extra-images.txt")

    # Read the current licenses file
    licenses_file = Path("licenses.d2iq.yaml")
    if not licenses_file.exists():
        print(f"Error: {licenses_file} not found")
        return 1

    with open(licenses_file) as f:
        licenses_content = f.read()

    # Track updates
    updated_count = 0
    new_count = 0

    for image_ref in extra_images:
        base_image = extract_base_image_name(image_ref)

        # Find existing entries for this base image
        escaped_base = re.escape(base_image)
        pattern = rf'(  - container_image: {escaped_base}(?::[^@\s]+)?(?:@[^\s]+)?\n(?:    [^\n]+\n)*)'

        match = re.search(pattern, licenses_content)
        if match:
            # Update existing entry
            new_entry_text = create_license_entry_text(image_ref, version)
            licenses_content = licenses_content.replace(match.group(1), new_entry_text)
            updated_count += 1
            print(f"Updated: {base_image}")
        else:
            # Add new image after the last KNative entry
            new_entry_text = create_license_entry_text(image_ref, version)

            knative_pattern = r'(  - container_image: gcr\.io/knative-releases/[^\n]+\n(?:    [^\n]+\n)*)'
            matches = list(re.finditer(knative_pattern, licenses_content))
            if matches:
                last_match = matches[-1]
                insertion_point = last_match.end()
                licenses_content = licenses_content[:insertion_point] + new_entry_text + licenses_content[insertion_point:]
                new_count += 1
                print(f"Added new: {base_image}")
            else:
                print(f"Could not find insertion point for: {base_image}")

    # Write updated file
    if updated_count > 0 or new_count > 0:
        with open(licenses_file, 'w') as f:
            f.write(licenses_content)

        print(f"\nUpdated {licenses_file}:")
        print(f"  - Updated {updated_count} existing entries")
        print(f"  - Added {new_count} new entries")
        print(f"  - Used version: knative-v{version}")
    else:
        print("\nNo changes made to licenses file.")

    return 0

if __name__ == "__main__":
    sys.exit(main())
