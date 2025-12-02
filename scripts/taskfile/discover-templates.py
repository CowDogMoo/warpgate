#!/usr/bin/env python3
"""
Warpgate Template Discovery Script

This script discovers Packer templates from multiple configured sources
and generates a JSON manifest for use in CI/CD workflows and local builds.

Usage:
    python scripts/discover-templates.py [options]

Options:
    --config PATH       Path to warpgate-config.yaml (default: ./warpgate-config.yaml)
    --output PATH       Path to output JSON file (default: ./discovered-templates.json)
    --format FORMAT     Output format: json, yaml, or github-matrix (default: json)
    --validate          Validate discovered templates for required files
    --verbose           Enable verbose output
"""

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Dict, List, Any, Optional
import yaml


class TemplateDiscovery:
    """Discovers and validates Packer templates from multiple sources."""

    def __init__(self, config_path: str, verbose: bool = False):
        self.config_path = Path(config_path)
        self.verbose = verbose
        self.config = self._load_config()
        self.discovered_templates: List[Dict[str, Any]] = []

    def _load_config(self) -> Dict[str, Any]:
        """Load the Warpgate configuration file."""
        if not self.config_path.exists():
            raise FileNotFoundError(f"Configuration file not found: {self.config_path}")

        with open(self.config_path, 'r') as f:
            config = yaml.safe_load(f)

        if not config or 'template_sources' not in config:
            raise ValueError("Invalid configuration: 'template_sources' not found")

        return config

    def _log(self, message: str):
        """Print verbose log message."""
        if self.verbose:
            print(f"[DISCOVERY] {message}", file=sys.stderr)

    def _expand_path(self, path: str) -> Path:
        """Expand environment variables and user home directory in path."""
        expanded = os.path.expandvars(os.path.expanduser(path))
        return Path(expanded)

    def _is_valid_template(self, template_path: Path) -> bool:
        """Check if a directory contains a valid Packer template."""
        required_files = self.config.get('discovery', {}).get('required_files', [
            'docker.pkr.hcl',
            'variables.pkr.hcl',
            'plugins.pkr.hcl'
        ])

        for required_file in required_files:
            if not (template_path / required_file).exists():
                self._log(f"Template at {template_path} missing required file: {required_file}")
                return False

        return True

    def _should_exclude(self, template_name: str) -> bool:
        """Check if template should be excluded based on patterns."""
        exclude_patterns = self.config.get('discovery', {}).get('exclude_patterns', [])

        for pattern in exclude_patterns:
            if pattern.startswith('*'):
                # Suffix match
                if template_name.endswith(pattern[1:]):
                    return True
            elif pattern.endswith('*'):
                # Prefix match
                if template_name.startswith(pattern[:-1]):
                    return True
            else:
                # Exact match
                if template_name == pattern:
                    return True

        return False

    def _get_template_metadata(self, template_name: str, template_path: Path, source_name: str) -> Dict[str, Any]:
        """Extract metadata for a discovered template."""
        build_config = self.config.get('build_config', {})
        template_overrides = build_config.get('template_overrides', {})

        # Get template-specific config or use defaults
        template_config = template_overrides.get(template_name, {})

        metadata = {
            'name': template_name,
            'path': str(template_path.absolute()),
            'source': source_name,
            'namespace': template_config.get('namespace', build_config.get('default_namespace', 'l50')),
            'vars': template_config.get('vars', f'template_name={template_name}'),
        }

        # Check for optional files
        optional_files = self.config.get('discovery', {}).get('optional_files', [])
        metadata['has_ami'] = (template_path / 'ami.pkr.hcl').exists()
        metadata['has_locals'] = (template_path / 'locals.pkr.hcl').exists()
        metadata['has_readme'] = (template_path / 'README.md').exists()

        return metadata

    def discover_from_source(self, source: Dict[str, Any]) -> List[Dict[str, Any]]:
        """Discover templates from a single source."""
        source_name = source.get('name', 'unknown')
        source_path = self._expand_path(source['path'])
        source_type = source.get('type', 'local')
        enabled = source.get('enabled', True)

        if not enabled:
            self._log(f"Source '{source_name}' is disabled, skipping")
            return []

        if not source_path.exists():
            self._log(f"Source path does not exist: {source_path}")
            return []

        if not source_path.is_dir():
            self._log(f"Source path is not a directory: {source_path}")
            return []

        self._log(f"Discovering templates from source '{source_name}' at {source_path}")

        discovered = []

        # Iterate through directories in the source path
        for item in source_path.iterdir():
            if not item.is_dir():
                continue

            template_name = item.name

            # Skip excluded templates
            if self._should_exclude(template_name):
                self._log(f"Excluding template '{template_name}' (matches exclude pattern)")
                continue

            # Validate template structure
            if not self._is_valid_template(item):
                self._log(f"Skipping '{template_name}' (invalid template structure)")
                continue

            # Extract metadata
            metadata = self._get_template_metadata(template_name, item, source_name)
            discovered.append(metadata)
            self._log(f"Discovered template: {template_name}")

        return discovered

    def discover_all(self) -> List[Dict[str, Any]]:
        """Discover templates from all configured sources."""
        self._log("Starting template discovery")

        sources = self.config.get('template_sources', [])
        all_templates = []

        for source in sources:
            templates = self.discover_from_source(source)
            all_templates.extend(templates)

        self._log(f"Discovery complete: found {len(all_templates)} templates")
        self.discovered_templates = all_templates
        return all_templates

    def generate_github_matrix(self) -> Dict[str, Any]:
        """Generate GitHub Actions matrix configuration."""
        architectures = self.config.get('build_config', {}).get('architectures', ['amd64', 'arm64'])

        # Build template list for matrix
        templates = []
        for template in self.discovered_templates:
            templates.append({
                'name': template['name'],
                'namespace': template['namespace'],
                'vars': template['vars']
            })

        # Build architecture list for matrix
        arch_configs = []
        for arch in architectures:
            if arch == 'amd64':
                arch_configs.append({
                    'arch': 'amd64',
                    'runner': 'ubuntu-latest',
                    'platform': 'linux/amd64'
                })
            elif arch == 'arm64':
                arch_configs.append({
                    'arch': 'arm64',
                    'runner': 'ubuntu-24.04-arm',
                    'platform': 'linux/arm64'
                })

        return {
            'template': templates,
            'architecture': arch_configs
        }

    def write_output(self, output_path: str, output_format: str = 'json'):
        """Write discovered templates to output file."""
        output_file = Path(output_path)

        if output_format == 'json':
            with open(output_file, 'w') as f:
                json.dump(self.discovered_templates, f, indent=2)
        elif output_format == 'yaml':
            with open(output_file, 'w') as f:
                yaml.dump(self.discovered_templates, f, default_flow_style=False)
        elif output_format == 'github-matrix':
            matrix = self.generate_github_matrix()
            with open(output_file, 'w') as f:
                json.dump(matrix, f, indent=2)
        else:
            raise ValueError(f"Unsupported output format: {output_format}")

        self._log(f"Wrote output to {output_file}")


def main():
    parser = argparse.ArgumentParser(
        description='Discover Warpgate Packer templates from multiple sources'
    )
    parser.add_argument(
        '--config',
        default='./warpgate-config.yaml',
        help='Path to warpgate-config.yaml (default: ./warpgate-config.yaml)'
    )
    parser.add_argument(
        '--output',
        default='./discovered-templates.json',
        help='Path to output file (default: ./discovered-templates.json)'
    )
    parser.add_argument(
        '--format',
        choices=['json', 'yaml', 'github-matrix'],
        default='json',
        help='Output format (default: json)'
    )
    parser.add_argument(
        '--validate',
        action='store_true',
        help='Validate discovered templates'
    )
    parser.add_argument(
        '--verbose',
        action='store_true',
        help='Enable verbose output'
    )

    args = parser.parse_args()

    try:
        discovery = TemplateDiscovery(args.config, verbose=args.verbose)
        templates = discovery.discover_all()

        if not templates:
            print("Warning: No templates discovered", file=sys.stderr)
            sys.exit(0)

        discovery.write_output(args.output, args.format)

        # Print summary
        print(f"Discovered {len(templates)} templates:")
        for template in templates:
            print(f"  - {template['name']} (from {template['source']})")

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
