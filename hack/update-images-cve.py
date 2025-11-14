#!/usr/bin/env python3
"""
Update container image tags in licenses.d2iq.yaml based on CVE scans.

This script:
1. Extracts all container_image entries from licenses.d2iq.yaml
2. Excludes images with dynamic tags (containing ${...})
3. Scans each image for CVEs using Trivy
4. Checks for newer versions and if having better CVSS score, proceed to update the image tag
5. Updates the YAML file with fixed versions

Prerequisites:
    - Trivy must be installed and available in PATH
      Install from: https://aquasecurity.github.io/trivy/latest/getting-started/installation/
    - skopeo must be installed and available in PATH
      Install from: https://github.com/containers/skopeo
    - Docker registry access (for checking available tags)

Usage:
    python3 hack/update-images-cve.py
"""

import re
import subprocess
import sys
import json
from pathlib import Path
from typing import Dict, List, Optional, Tuple


def is_dynamic_tag(tag: str) -> bool:
    return bool(re.search(r'\$\{[^}]+\}', tag))


def parse_image_reference(image_ref: str) -> Tuple[str, str, Optional[str]]:
    """Parse a Docker image reference into registry, repository, and tag."""
    if ':' in image_ref:
        parts = image_ref.rsplit(':', 1)
        image_part = parts[0]
        tag = parts[1]
    else:
        image_part = image_ref
        tag = None

    # Determine registry and repository
    parts = image_part.split('/', 1)
    if '.' in parts[0] or ':' in parts[0]: # . for domain, : for port
        registry = parts[0]
        repository = parts[1]
    else:
        registry = 'docker.io'
        repository = image_part

    return registry, repository, tag


def scan_image_cves(image_ref: str) -> Tuple[bool, List[Dict]]:
    """
    Scan an image for CVEs using Trivy.
    Returns (has_cves, vulnerabilities_list).
    """
    try:
        result = subprocess.run(
            ['trivy', 'image', '--format', 'json', '--no-progress', '--quiet', image_ref],
            capture_output=True,
            text=True,
            timeout=300
        )

        if result.returncode != 0:
            print(f"  Warning: Trivy scan failed for {image_ref}: {result.stderr}")
            return False, []

        data = json.loads(result.stdout)
        vulnerabilities = []

        if 'Results' in data:
            for result_item in data['Results']:
                if 'Vulnerabilities' in result_item:
                    vulnerabilities.extend(result_item['Vulnerabilities'])

        return len(vulnerabilities) > 0, vulnerabilities

    except subprocess.TimeoutExpired:
        print(f"  Warning: Trivy scan timed out for {image_ref}")
        return False, []
    except json.JSONDecodeError:
        print(f"  Warning: Failed to parse Trivy output for {image_ref}")
        return False, []
    except FileNotFoundError:
        print("Error: Trivy not found. Please install Trivy: https://aquasecurity.github.io/trivy/latest/getting-started/installation/")
        sys.exit(1)
    except Exception as e:
        print(f"  Warning: Error scanning {image_ref}: {e}")
        return False, []


def get_cvss_score(vulnerability: Dict) -> float:
    """
    Extract CVSS score from Trivy vulnerability data.
    Returns CVSS score (0.0-10.0) or None if not available.
    """
    # Try CVSS v3 first (most common)
    cvss = vulnerability.get('CVSS', {})
    if isinstance(cvss, dict):
        for source in ['nvd', 'redhat', 'ghsa', 'bitnami']:
            if source in cvss:
                v3_score = cvss[source].get('V3Score')
                if v3_score is not None:
                    return float(v3_score)
                v2_score = cvss[source].get('V2Score')
                if v2_score is not None:
                    return float(v2_score)

    # Fallback: check if score is directly in CVSS
    if isinstance(cvss, dict):
        v3_score = cvss.get('V3Score')
        if v3_score is not None:
            return float(v3_score)
        v2_score = cvss.get('V2Score')
        if v2_score is not None:
            return float(v2_score)

    return None


def get_highest_cve_score(vulnerabilities: List[Dict]) -> float:
    """
    Get the highest CVSS score from vulnerabilities.
    Returns the highest CVSS score (0.0-10.0), or falls back to severity mapping.
    """
    if not vulnerabilities:
        return 0.0

    max_score = 0.0
    for vuln in vulnerabilities:
        # Try to get CVSS score first
        cvss_score = get_cvss_score(vuln)
        if cvss_score is not None:
            if cvss_score > max_score:
                max_score = cvss_score
        else:
            # Fallback to severity mapping if CVSS not available
            severity = vuln.get('Severity', 'UNKNOWN')
            score = get_severity_score(severity)
            if score > max_score:
                max_score = score

    return max_score


def get_severity_score(severity: str) -> float:
    """
    Map severity level to CVSS-equivalent score (0-10 scale).
    Used as fallback when CVSS score is not available.
    """
    severity_scores = {
        'CRITICAL': 10.0,
        'HIGH': 8.0,
        'MEDIUM': 6.0,
        'LOW': 3.0,
        'UNKNOWN': 1.0
    }
    return severity_scores.get(severity.upper(), 1.0)


def get_severity_from_cvss_score(score: float) -> str:
    """Convert CVSS score to severity level."""
    if score >= 9.0:
        return 'CRITICAL'
    elif score >= 7.0:
        return 'HIGH'
    elif score >= 4.0:
        return 'MEDIUM'
    elif score >= 0.1:
        return 'LOW'
    else:
        return 'UNKNOWN'


def is_valid_tag(tag: str) -> bool:
    """
    Check if a tag is valid (not a digest-based or platform-specific tag).
    Filters out tags that start with 'sha256', 'x86_64', or 'windows'.
    Returns True if tag is valid, False otherwise.
    """
    # Exclude sha256-prefixed tags (digest-based tags)
    # Exclude x86_64 and windows platform-specific tags
    return not (
      tag.startswith('sha256') or
      tag.startswith('x86_64') or
      tag.startswith('windows') or
      tag.startswith('arm') or
      tag.startswith('aarch64') or
      tag.find('-beta') != -1 or
      tag.find('-debug') != -1
    )


def matches_version_pattern(tag: str) -> bool:
    """
    Check if a tag matches the pattern [v]X[.Y][.Z][-s] where:
    - v is optional 'v' prefix
    - X is a required number
    - .Y is optional dot followed by number
    - .Z is optional dot followed by number
    - -s is optional dash followed by non-empty string
    """
    pattern = r'^(v?\d+(\.\d+)?(\.\d+)?(-.+)?)$'
    return bool(re.match(pattern, tag))


def extract_version_numbers(tag: str) -> Optional[List[int]]:
    """
    Extract version numbers from a tag.
    Returns list of integers [X, Y, Z] or [X, Y] or [X], or None if not a version tag.
    """
    # Strip optional 'v' prefix
    tag_normalized = tag.lstrip('v')

    # Extract version part before any suffix (e.g., "1.2.3-alpine" -> "1.2.3")
    version_part = re.split(r'[-_\s]', tag_normalized)[0]

    # Try to parse as version numbers
    parts = version_part.split('.')
    version_nums = []

    for part in parts:
        try:
            version_nums.append(int(part))
        except ValueError:
            # If any part is not numeric, return None
            return None

    return version_nums if version_nums else None


def compare_version_tags(tag1: str, tag2: str) -> int:
    """
    Compare two version tags.
    Returns: -1 if tag1 < tag2, 0 if tag1 == tag2, 1 if tag1 > tag2
    """
    v1_nums = extract_version_numbers(tag1)
    v2_nums = extract_version_numbers(tag2)

    if v1_nums is None or v2_nums is None:
        # Fallback to string comparison if not version tags
        if tag1 < tag2:
            return -1
        elif tag1 > tag2:
            return 1
        return 0

    # Pad shorter version with zeros
    max_len = max(len(v1_nums), len(v2_nums))
    v1_nums.extend([0] * (max_len - len(v1_nums)))
    v2_nums.extend([0] * (max_len - len(v2_nums)))

    for p1, p2 in zip(v1_nums, v2_nums):
        if p1 < p2:
            return -1
        elif p1 > p2:
            return 1
    return 0


def get_latest_tag_by_version_sorting(valid_tags: List[str], current_tag: Optional[str] = None) -> Optional[str]:
    """
    Fallback function to get the latest tag by sorting available tags in version order.
    Filters tags matching version pattern, sorts them, and returns the first (highest) tag.

    Returns the highest version tag after sorting, or None if no valid tags.
    """
    if not valid_tags:
        return None

    # Filter tags that match version pattern
    version_tags = [tag for tag in valid_tags if matches_version_pattern(tag)]

    if not version_tags:
        return None

    # Sort tags in reverse order (highest version first)
    sorted_tags = sorted(
        version_tags,
        key=lambda t: extract_version_numbers(t) or [0],
        reverse=True
    )

    if sorted_tags:
        print(f"    Selected latest tag by version sorting: {sorted_tags[0]}")
        return sorted_tags[0]

    return None


def get_latest_tag(registry: str, repository: str, current_tag: Optional[str] = None) -> Optional[str]:
    """
    Get the latest version tag for a repository using skopeo.

    This function:
    1. Gets the digest of the 'latest' tag using skopeo inspect
    2. Lists all tags using skopeo list-tags
    3. Filters out sha256-prefixed tags (digest-based tags)
    4. Checks all remaining tags in order, gets each tag's digest and compares with latest digest
    5. Returns the first tag that matches the 'latest' tag digest
    6. If latest tag is not found, falls back to version-based sorting:
       - Filters tags matching version pattern, sorts them, and returns the highest one

    Prerequisites:
        - skopeo binary must be installed and available in PATH

    Returns the first tag that matches the 'latest' tag digest, or the highest version tag if latest not found.
    """
    try:
        # Check if skopeo is available
        try:
            subprocess.run(['skopeo', '--version'], capture_output=True, check=True, timeout=5)
        except (subprocess.CalledProcessError, FileNotFoundError, subprocess.TimeoutExpired):
            print(f"    Warning: skopeo not found. Please install skopeo: https://github.com/containers/skopeo")
            return None

        image_url = f"docker://{registry}/{repository}"

        latest_digest = None
        try:
            latest_image_url = f"{image_url}:latest"
            result = subprocess.run(
                ['skopeo', 'inspect', '--override-arch', 'amd64', '--override-os', 'linux',
                 latest_image_url, '--format', '{{.Digest}}'],
                capture_output=True,
                text=True,
                timeout=30
            )

            if result.returncode == 0:
                latest_digest = result.stdout.strip()
                print(f"    Digest of latest tag: {latest_digest}")
            else:
                print(f"    Warning: Failed to inspect latest tag: {result.stderr}")
        except subprocess.TimeoutExpired:
            print(f"    Warning: Timeout while inspecting latest tag")
        except Exception as e:
            print(f"    Warning: Error inspecting latest tag: {e}")

        all_tags = []
        try:
            result = subprocess.run(
                ['skopeo', 'list-tags', '--override-arch', 'amd64', '--override-os', 'linux', image_url],
                capture_output=True,
                text=True,
                timeout=30
            )

            if result.returncode != 0:
                print(f"    Warning: Failed to list tags: {result.stderr}")
                return None

            try:
                data = json.loads(result.stdout)
                all_tags = data.get('Tags', [])
                print(f" Received {len(all_tags)} tags")
            except json.JSONDecodeError:
                print(f"    Warning: Failed to parse tags output as JSON: {result.stderr}")
                return None
        except subprocess.TimeoutExpired:
            print(f"    Warning: Timeout while listing tags")
            return None
        except Exception as e:
            print(f"    Warning: Error listing tags: {e}")
            return None

        if not all_tags:
            print(f"    Warning: No tags found")
            return None

        # Step 3: Filter out sha256-prefixed tags (digest-based tags)
        valid_tags = [tag for tag in all_tags if is_valid_tag(tag)]
        print(f" Post Validation: Checking in {len(valid_tags)} tags")

        if not valid_tags:
            print(f"    Warning: No valid tags found (all tags are sha256 digest-based)")
            return None

        # If latest_digest is not available, fall back to version-based sorting
        if not latest_digest:
            print(f"    Latest tag not available, falling back to version-based sorting")
            return get_latest_tag_by_version_sorting(valid_tags, current_tag)

        # Step 4: If current tag matches version pattern, filter and sort tags
        tags_to_check = valid_tags
        if current_tag and matches_version_pattern(current_tag):
            # Filter to only tags greater than current version
            tags_to_check = [
                tag for tag in valid_tags
                if matches_version_pattern(tag) and compare_version_tags(tag, current_tag) > 0
            ]
            print(f" Filtered to {len(tags_to_check)} tags greater than current version {current_tag}")

            # Sort in reverse order (highest version first)
            # Use a tuple with a large number for None values to push them to the end
            tags_to_check = sorted(
                tags_to_check,
                key=lambda t: extract_version_numbers(t) or [0],
                reverse=True
            )
            print(f" Sorted to {tags_to_check}")
        else:
            # If current tag doesn't match pattern, check all tags in original order
            print(f" Current tag '{current_tag}' doesn't match version pattern, checking all tags")

        # Step 5: Check tags in order, return the first match
        # For each tag, get its digest and compare with latest digest
        # Return the first match found
        for tag in tags_to_check:
            try:
                tag_image_url = f"{image_url}:{tag}"
                result = subprocess.run(
                    ['skopeo', 'inspect', '--override-arch', 'amd64', '--override-os', 'linux',
                     tag_image_url, '--format', '{{.Digest}}'],
                    capture_output=True,
                    text=True,
                    timeout=30
                )

                if result.returncode == 0:
                    tag_digest = result.stdout.strip()
                    if tag_digest == latest_digest:
                        return tag
            except subprocess.TimeoutExpired:
                # Continue to next tag if timeout
                continue
            except Exception as e:
                # Continue to next tag if error
                continue

        # No matching tag found - fall back to version-based sorting
        print(f"    Warning: No tag found matching latest digest, falling back to version-based sorting")
        return get_latest_tag_by_version_sorting(valid_tags, current_tag)

    except Exception as e:
        print(f"    Warning: Failed to fetch latest tag using skopeo: {e}")
        return None


def extract_images_from_yaml(yaml_content: str) -> List[Dict]:
    """Extract container_image entries from YAML content using simple line-by-line parsing."""
    images = []
    lines = yaml_content.split('\n')

    for i, line in enumerate(lines):
        # Look for container_image line
        match = re.match(r'^(\s*)(-+\s*)container_image:\s+(.+)$', line)
        if match:
            indent = match.group(1)
            dash_part = match.group(2) or ''
            image_ref = match.group(3).strip()

            # Skip dynamic tags
            if is_dynamic_tag(image_ref):
                continue

            images.append({
                'image_ref': image_ref,
                'original_indent': indent,
                'dash_format': dash_part,
                'line_number': i + 1,
                'original_line': line
            })

    return images


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
    with open(licenses_file) as f:
        licenses_content = f.read()

    static_images = extract_images_from_yaml(licenses_content)

    if not static_images:
        print("No images with static tags found.")
        return 0

    print(f"Scanning {len(static_images)} images for CVEs...")

    updates = []
    scanned_count = 0

    for img_info in static_images:
        image_ref = img_info['image_ref']
        scanned_count += 1

        print(f"\n[{scanned_count}/{len(static_images)}] Scanning {image_ref}...")

        has_cves, vulnerabilities = scan_image_cves(image_ref)

        if not has_cves:
            print(f"{image_ref}: No CVEs found")
            continue

        current_highest_score = get_highest_cve_score(vulnerabilities)
        print(f"Current highest CVSS score: {current_highest_score:.1f} ({get_severity_from_cvss_score(current_highest_score)})")

        registry, repository, tag = parse_image_reference(image_ref)

        print(f"{image_ref}: Registry: {registry}, Repository: {repository}, Tag: {tag}")

        # Get latest tag version
        print(f"{image_ref}: Fetching latest tag from {registry}...")
        latest_tag = get_latest_tag(registry, repository, current_tag=tag)

        if not latest_tag:
            print(f"  Could not fetch latest tag. Skipping automatic update.")
            updates.append({
                'image_ref': image_ref,
                'vulnerabilities': vulnerabilities,
                'registry': registry,
                'repository': repository,
                'current_tag': tag,
                'new_tag': None,
                'reason': 'Could not fetch latest tag'
            })
            continue

        print(f"{image_ref}: Latest tag available: {latest_tag}")

        # Build new image reference
        new_image_ref = image_ref.replace(f":{tag}", f":{latest_tag}")

        print(f"  Checking latest version: {new_image_ref}")

        # Scan the latest version for CVEs
        _, vulnerabilities_new = scan_image_cves(new_image_ref)

        # Calculate CVE scores for current and latest versions
        latest_highest_score = get_highest_cve_score(vulnerabilities_new)

        print(f"  Latest highest CVSS score: {latest_highest_score:.1f} ({get_severity_from_cvss_score(latest_highest_score)})")

        # Decision logic based on CVSS scores
        should_update = False
        reason = ''

        if current_highest_score == 0.0:
            # Current has no CVEs, no need to update
            print(f"  Current version has no CVEs, no update needed")
            continue
        elif latest_highest_score < current_highest_score:
            # Latest has lower highest score - always update
            should_update = True
            reason = f'Latest has lower highest CVSS score ({latest_highest_score:.1f} < {current_highest_score:.1f})'
            print(f"  ✓ {reason}")
        else:
            # Latest has higher score - don't update
            should_update = False
            reason = f'Latest has higher highest CVSS score ({latest_highest_score:.1f} >= {current_highest_score:.1f})'
            print(f"  {reason}")

        if should_update:
            # Found a fix!
            updates.append({
                'image_ref': image_ref,
                'new_image_ref': new_image_ref,
                'vulnerabilities': vulnerabilities,
                'registry': registry,
                'repository': repository,
                'current_tag': tag,
                'new_tag': latest_tag,
                'reason': 'Fixed'
            })
        else:
            updates.append({
                'image_ref': image_ref,
                'vulnerabilities': vulnerabilities,
                'registry': registry,
                'repository': repository,
                'current_tag': tag,
                'new_tag': latest_tag,
                'reason': reason
            })

    # Summary
    print(f"\n{'='*60}")
    print("Summary:")
    print(f"  Total images scanned: {scanned_count}")
    print(f"  Images with CVEs: {len(updates)}")

    if updates:
        print(f"\nImages with CVEs:")
        fixed_count = 0
        for update in updates:
            print(f"  - {update['image_ref']}")
            print(f"    Vulnerabilities: {len(update['vulnerabilities'])}")

            if update.get('new_tag'):
                if update.get('reason') == 'Fixed':
                    print(f"    → Will update to: {update['new_image_ref']}")
                    fixed_count += 1
                else:
                    print(f"    → Newer version available: {update['new_tag']} ({update.get('reason', 'N/A')})")
            else:
                print(f"    → No fix available ({update.get('reason', 'N/A')})")

            # Show top 3 CVEs
            for vuln in update['vulnerabilities'][:3]:
                severity = vuln.get('Severity', 'UNKNOWN')
                vuln_id = vuln.get('VulnerabilityID', 'N/A')
                title = vuln.get('Title', 'N/A')
                print(f"      [{severity}] {vuln_id}: {title[:60]}")
            if len(update['vulnerabilities']) > 3:
                print(f"      ... and {len(update['vulnerabilities']) - 3} more")

    # Apply updates
    updates_to_apply = [u for u in updates if u.get('reason') == 'Fixed']

    if updates_to_apply:
        print(f"\nApplying {len(updates_to_apply)} updates to {licenses_file}...")
        updated_content = licenses_content

        for update in updates_to_apply:
            old_image_ref = update['image_ref']
            new_image_ref = update['new_image_ref']

            updated_content = update_image_tag_in_content(
                updated_content,
                old_image_ref,
                new_image_ref
            )
            print(f"  Updated: {old_image_ref} → {new_image_ref}")

        # Write updated file
        with open(licenses_file, 'w') as f:
            f.write(updated_content)

        print(f"\n✓ Successfully updated {licenses_file}")
        print(f"  Updated {len(updates_to_apply)} image tags")
    else:
        print("\nNo updates to apply.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
