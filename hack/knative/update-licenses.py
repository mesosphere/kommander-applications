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


def create_license_entry_text(image_ref, version):
    """Create the text for a license entry."""
    repo_url = get_repository_url(image_ref)

    # For operator images, use ${image_tag} variable instead of hardcoded version
    if 'knative.dev/operator' in image_ref:
        ref_value = "knative-${image_tag}"
    elif 'knative.dev/pkg' in image_ref:
        # pkg repository uses release branches without 'v' prefix and no patch version
        major_minor = '.'.join(version.split('.')[:2])  # e.g., "1.19.1" -> "1.18"
        ref_value = f"release-{major_minor}"
    else:
        ref_value = f"knative-v{version}"

    return (f"  - container_image: {image_ref}\n"
            f"    sources:\n"
            f"      - license_path: LICENSE\n"
            f"        ref: {ref_value}\n"
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

    # Add operator images (they are not in extra-images.txt)
    operator_images = [
        f"gcr.io/knative-releases/knative.dev/operator/cmd/operator:v{version}",
        f"gcr.io/knative-releases/knative.dev/operator/cmd/webhook:v{version}"
    ]

    # Combine all images - operator images first, then extra images
    all_knative_images = operator_images + extra_images

    # Read the current licenses file
    licenses_file = Path("licenses.d2iq.yaml")
    if not licenses_file.exists():
        print(f"Error: {licenses_file} not found")
        return 1

    with open(licenses_file) as f:
        licenses_content = f.read()

    # Remove ALL existing KNative entries
    knative_pattern = r'(  - container_image: gcr\.io/knative-releases/[^\n]+\n(?:    [^\n]+\n)*)'
    existing_matches = list(re.finditer(knative_pattern, licenses_content))

    removed_count = len(existing_matches)
    for match in reversed(existing_matches):  # Remove in reverse order to maintain indices
        licenses_content = licenses_content[:match.start()] + licenses_content[match.end():]

    if removed_count > 0:
        print(f"Removed {removed_count} existing KNative entries")

    # Generate new license entries for all images
    new_knative_entries = ""
    for image_ref in all_knative_images:
        new_knative_entries += create_license_entry_text(image_ref, version)

    # Find insertion point (after Velero images, before other images)
    velero_pattern = r'(  - container_image: docker\.io/velero/velero:v[^\n]+\n(?:    [^\n]+\n)*)'
    velero_match = re.search(velero_pattern, licenses_content)

    if velero_match:
        insertion_point = velero_match.end()
        licenses_content = licenses_content[:insertion_point] + new_knative_entries + licenses_content[insertion_point:]
        print(f"Added {len(all_knative_images)} KNative entries after Velero")
    else:
        print("Could not find Velero entries as insertion point")
        return 1

    # Write updated file
    with open(licenses_file, 'w') as f:
        f.write(licenses_content)

    print(f"\nUpdated {licenses_file}:")
    print(f"  - Removed {removed_count} existing KNative entries")
    print(f"  - Added {len(all_knative_images)} new KNative entries")
    print(f"  - Used version: knative-v{version}")
    print(f"  - Operator images: {len(operator_images)}")
    print(f"  - Extra images: {len(extra_images)}")

    return 0

if __name__ == "__main__":
    sys.exit(main())
