#!/usr/bin/env python3
"""
Resolve Template Directory Script

This script resolves the directory path for a given Packer template name,
checking the discovered-templates.json file first, then falling back to
the default packer-templates directory.

Usage:
    python scripts/taskfile/resolve-template-dir.py --template-name <name> [--template-dir <dir>]

Options:
    --template-name NAME    Name of the template to resolve (required if --template-dir not provided)
    --template-dir DIR      Explicitly provided template directory (takes precedence)
    --discovered-file PATH  Path to discovered templates JSON (default: ./discovered-templates.json)
"""

import argparse
import json
import sys
from pathlib import Path
from typing import Optional


def resolve_template_dir(
    template_name: Optional[str] = None,
    template_dir: Optional[str] = None,
    discovered_file: str = "discovered-templates.json"
) -> str:
    """
    Resolve the template directory path.

    Priority:
    1. If template_dir is explicitly provided, use it
    2. Look up template_name in discovered-templates.json
    3. Fall back to packer-templates/{template_name}

    Args:
        template_name: Name of the template to find
        template_dir: Explicitly provided directory path
        discovered_file: Path to the discovered templates JSON file

    Returns:
        Resolved template directory path
    """
    # Priority 1: Explicit template directory
    if template_dir:
        return template_dir

    # Priority 2: Look up in discovered templates
    discovered_path = Path(discovered_file)
    if discovered_path.exists():
        try:
            with open(discovered_path, 'r') as f:
                templates = json.load(f)

            for template in templates:
                if template.get('name') == template_name:
                    return template['path']
        except (json.JSONDecodeError, KeyError, IOError):
            # If we can't parse or access the file, continue to fallback
            pass

    # Priority 3: Default location
    if template_name:
        return f"packer-templates/{template_name}"

    # If neither template_name nor template_dir provided, use current directory
    return "./"


def main():
    parser = argparse.ArgumentParser(
        description="Resolve Packer template directory path",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )

    parser.add_argument(
        "--template-name",
        help="Name of the template to resolve"
    )

    parser.add_argument(
        "--template-dir",
        help="Explicitly provided template directory (takes precedence)"
    )

    parser.add_argument(
        "--discovered-file",
        default="discovered-templates.json",
        help="Path to discovered templates JSON file (default: ./discovered-templates.json)"
    )

    args = parser.parse_args()

    # Validate inputs
    if not args.template_dir and not args.template_name:
        print("Error: Either --template-name or --template-dir must be provided", file=sys.stderr)
        sys.exit(1)

    # Resolve and print the template directory
    resolved_dir = resolve_template_dir(
        template_name=args.template_name,
        template_dir=args.template_dir,
        discovered_file=args.discovered_file
    )

    print(resolved_dir)
    return 0


if __name__ == "__main__":
    sys.exit(main())
