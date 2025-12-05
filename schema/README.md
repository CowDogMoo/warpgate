# Warpgate JSON Schema

This directory contains the JSON Schema for Warpgate template files.

## Usage

To enable IDE autocomplete and validation for your Warpgate templates, add
this line at the top of your `warpgate.yaml` files:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/cowdogmoo/warpgate/main/schema/warpgate-template.json
```

Or if working locally:

```yaml
# yaml-language-server: $schema=../schema/warpgate-template.json
```

## Generating the Schema

The schema is automatically generated from the Go struct definitions in `pkg/builder/config.go`.

To regenerate the schema:

```bash
task schema:generate
```

## CI Validation

The schema should always be kept in sync with the Go structs. Run this before committing:

```bash
task schema:validate
```

This task is run automatically in CI to ensure the schema matches the current code.

## Schema Details

- **Schema URL**: `https://warpgate.dev/schema/template.json`
- **JSON Schema Version**: Draft 2020-12
- **Generator**: [invopop/jsonschema](https://github.com/invopop/jsonschema)
