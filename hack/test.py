#!/usr/bin/env python3
"""
Test script to replace docker.io/grafana/grafana:12.1.0 with docker.io/grafana/grafana:12.2.1
in licenses.d2iq.yaml using the simple regex replace approach.
"""

import re
from pathlib import Path


def update_image_tag_in_content(content: str, old_image_ref: str, new_image_ref: str) -> str:
    """Update an image reference in the YAML content using simple regex replace.

    This preserves all formatting (indentation, dash format, trailing whitespace)
    and only replaces exact matches of the old_image_ref.
    """
    # Pattern captures:
    # \1 = leading whitespace (indentation)
    # \2 = dash format (-, - , -  -, etc.)
    # \3 = trailing whitespace (if any)
    # Only matches exact old_image_ref
    pattern = rf'^(\s*)(-+\s*)container_image:\s+{re.escape(old_image_ref)}(\s*)$'
    replacement = rf'\1\2container_image: {new_image_ref}\3'

    # Replace all occurrences (though each image:tag should be unique)
    updated_content = re.sub(pattern, replacement, content, flags=re.MULTILINE)

    return updated_content


def main():
    licenses_file = Path('licenses.d2iq.yaml')
    if not licenses_file.exists():
        print(f"Error: {licenses_file} not found")
        return 1

    # Read the file
    with open(licenses_file) as f:
        content = f.read()

    old_image_ref = 'docker.io/grafana/grafana:12.1.0'
    new_image_ref = 'docker.io/grafana/grafana:12.2.1'

    # Check if old image ref exists
    if old_image_ref not in content:
        print(f"Warning: {old_image_ref} not found in {licenses_file}")
        # Check what grafana versions exist
        grafana_lines = [line for line in content.split('\n') if 'grafana/grafana:' in line]
        if grafana_lines:
            print("\nFound grafana/grafana lines:")
            for line in grafana_lines[:5]:
                print(f"  {line}")
        return 1

    print(f"Replacing {old_image_ref} with {new_image_ref}...")

    # Perform the replacement
    updated_content = update_image_tag_in_content(content, old_image_ref, new_image_ref)

    # Check if replacement was successful
    if updated_content == content:
        print("Warning: No changes made. The pattern might not have matched.")
        return 1

    # Show the diff
    old_lines = content.split('\n')
    new_lines = updated_content.split('\n')
    print("\nChanges:")
    for i, (old_line, new_line) in enumerate(zip(old_lines, new_lines), 1):
        if old_line != new_line:
            print(f"  Line {i}:")
            print(f"    - {old_line}")
            print(f"    + {new_line}")

    # Write the updated file
    with open(licenses_file, 'w') as f:
        f.write(updated_content)

    print(f"\n✓ Successfully updated {licenses_file}")
    return 0


if __name__ == "__main__":
    import sys
    sys.exit(main())
