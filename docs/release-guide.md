# Release Management Guide

Automated version and release management for Warp Gate.

---

## 📋 Overview

This guide provides comprehensive instructions for creating and managing Warp Gate
releases, including:

- 🏷️ Version determination (semantic versioning)
- 🚀 Automated release workflows
- 📊 Release verification
- 🔍 Changelog generation
- 🐛 Troubleshooting

---

## 🚀 Quick Start

```bash
# 1. Preview changes since last release
task go:release-changelog FROM=$CURRENT_VERSION

# 2. Create and push release tag (triggers goreleaser GitHub Action)
task go:release TAG=$NEXT_VERSION

# 3. Wait for the release build (takes 5-10 minutes)
task go:release-watch

# 4. Verify release once the release build completes
task go:release-check TAG=$NEXT_VERSION
```

**⏱️ Important Timing Notes:**

- **Step 2** only creates the git tag - it does NOT create the release
- **Step 3** is where goreleaser builds all binaries (5-10 minutes)
- **Step 4** will show "release not found" if you run it before step 3 finishes
- The release appears on GitHub only after all binaries are built and uploaded

---

## 📝 Determining the Next Version

Warp Gate follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (v3.0.0): Breaking changes, incompatible API changes
- **MINOR** (v2.1.0): New features, backwards compatible
- **PATCH** (v2.0.1): Bug fixes, backwards compatible

### Version Decision Matrix

| Change Type | Example | Version |
| --- | --- | --- |
| Remove features/templates | Deleted legacy Packer templates | **MAJOR** |
| Change configuration format | New template YAML schema | **MAJOR** |
| Breaking API changes | Renamed CLI commands | **MAJOR** |
| New features/capabilities | Added new provisioner | **MINOR** |
| Non-breaking enhancements | Additional template options | **MINOR** |
| Bug fixes | Fixed build errors | **PATCH** |
| Documentation updates | Updated README | **PATCH** |
| Dependency updates (non-breaking) | Updated Go dependencies | **PATCH** |

### Checking What Changed

```bash
# View commits since last release
git log v2.0.1..HEAD --oneline

# View detailed changelog
task go:release-changelog FROM=v2.0.1

# Check commit stats
git log v2.0.1..HEAD --oneline | wc -l

# View file changes
git diff --stat v2.0.1..HEAD
```

---

## 🔄 Release Workflows

### Standard Release Process (Recommended)

This is the recommended workflow using Task automation:

```bash
# 1. Ensure working directory is clean
git status

# 2. Preview changelog
task go:release-changelog FROM=v2.0.1

# 3. Determine version (see Version Decision Matrix)
NEXT_VERSION=v3.0.0  # Update based on changes

# 4. (Optional) Test goreleaser locally
task go:release-test

# 5. Create and push release tag
task go:release TAG=$NEXT_VERSION

# 6. Monitor the release build
task go:release-watch

# 7. Verify release was created
task go:release-check TAG=$NEXT_VERSION
```

### Manual Release Process

If you prefer manual control:

```bash
# 1. Create annotated tag
git tag -a v3.0.0 -m "Release v3.0.0

Breaking Changes:
- Removed legacy Packer templates
- Enforced git-only repository configuration

See CHANGELOG for full details"

# 2. Push tag to trigger goreleaser
git push origin v3.0.0

# 3. Monitor workflow
gh run watch

# 4. Verify release
gh release view v3.0.0
```

### Hotfix Release

For urgent bug fixes:

```bash
# 1. Create hotfix branch
git checkout -b hotfix-v2.0.2

# 2. Make fixes and commit
git add .
git commit -m "fix: critical bug in template loader"

# 3. Merge to main
git checkout main
git merge hotfix-v2.0.2

# 4. Create patch release
task go:release TAG=v2.0.2

# 5. Delete hotfix branch
git branch -d hotfix-v2.0.2
```

### Pre-release (Beta/RC)

For testing before stable release:

```bash
# Create pre-release tag
git tag -a v3.0.0-beta.1 -m "Release v3.0.0-beta.1"
git push origin v3.0.0-beta.1

# This triggers goreleaser but marks as pre-release
```

---

## 📚 Task Reference

### `go:release`

Create and push a new release tag (triggers goreleaser GitHub Action).

```bash
task go:release TAG=v3.0.0
```

**What it does:**

- Creates an annotated git tag
- Pushes tag to remote (triggers goreleaser)
- Validates working directory is clean
- Requires gh CLI

**Preconditions:**

- Clean working directory (no uncommitted changes)
- No staged changes
- `gh` CLI installed
- `TAG` parameter provided

### `go:release-changelog`

Generate changelog between two tags.

```bash
# Changes since last tag
task go:release-changelog

# Changes between specific tags
task go:release-changelog FROM=v2.0.0 TO=v2.0.1

# Changes from tag to HEAD
task go:release-changelog FROM=v2.0.1 TO=HEAD
```

**Variables:**

- `FROM` (optional) - Starting tag (defaults to latest tag)
- `TO` (optional) - Ending ref (defaults to HEAD)

### `go:release-check`

Check if a release exists and view its status.

```bash
# List recent releases
task go:release-check

# View specific release
task go:release-check TAG=v2.0.1
```

### `go:release-watch`

Watch the goreleaser workflow run in real-time.

```bash
task go:release-watch
```

**Requires:** `gh` CLI

### `go:release-test`

Test goreleaser locally (snapshot build, no publish).

```bash
task go:release-test
```

**What it does:**

- Runs goreleaser in snapshot mode
- Creates binaries in `dist/` directory
- Does not publish anything
- Useful for testing `.goreleaser.yaml` changes

**Requires:** `goreleaser` installed locally

```bash
# Install goreleaser
brew install goreleaser
```

### `go:release-draft`

Create a draft release with gh CLI.

```bash
# With auto-generated notes
task go:release-draft TAG=v3.0.0

# With custom notes
task go:release-draft TAG=v3.0.0 NOTES="Custom release notes"

# With custom title
task go:release-draft TAG=v3.0.0 TITLE="Major Release v3.0.0"
```

**Variables:**

- `TAG` (required) - Release tag
- `TITLE` (optional) - Release title (defaults to TAG)
- `NOTES` (optional) - Custom notes (defaults to auto-generated)

**Note:** This bypasses goreleaser and creates a draft release manually.
Use this only if you need to preview/edit before publishing.

### `go:release-delete`

Delete a release and its tag.

```bash
task go:release-delete TAG=v1.0.0
```

**What it does:**

- Deletes GitHub release
- Deletes local git tag
- Deletes remote git tag
- Prompts for confirmation

**Warning:** This is destructive and should be used carefully.

---

## 🤖 GitHub Actions Integration

### How It Works

The release process is automated via GitHub Actions (`.github/workflows/goreleaser.yaml`):

```yaml
name: goreleaser
on:
  push:
    tags:
      - "*"
```

**Workflow:**

1. Tag push triggers the workflow
2. Fetches all tags and sets up Go 1.25.5
3. Runs goreleaser with `.goreleaser.yaml` config
4. Builds binaries for:
   - **OS**: linux, darwin
   - **Arch**: amd64, arm, arm64
   - **Variants**: arm6, arm7, amd64v2, amd64v3
5. Creates GitHub release with auto-generated notes
6. Uploads all build artifacts

### Viewing Workflow Runs

```bash
# List recent workflow runs
gh run list --workflow=goreleaser.yaml

# Watch latest run
gh run watch

# View specific run
gh run view <run-id>
```

---

## 📋 Release Checklist

Use this checklist for major releases:

```markdown
## Pre-Release

- [ ] All tests passing
- [ ] Documentation updated
- [ ] CHANGELOG reviewed
- [ ] Breaking changes documented
- [ ] Migration guide created (if needed)
- [ ] Pre-commit hooks pass
- [ ] Dependencies updated

## Release

- [ ] Determined correct version number
- [ ] Reviewed changelog (`task go:release-changelog`)
- [ ] Tested goreleaser locally (`task go:release-test`)
- [ ] Created and pushed tag (`task go:release TAG=vX.Y.Z`)
- [ ] Monitored release build (`task go:release-watch`)
- [ ] Verified release created (`task go:release-check`)

## Post-Release

- [ ] Tested installation (`go install github.com/CowDogMoo/warpgate/cmd/warpgate@latest`)
- [ ] Verified binaries work on different platforms
```

---

## 🏷️ Git Tag Strategy

### Tag Naming Convention

| Release Type | Tag Format | Example | When to Use |
| --- | --- | --- | --- |
| Stable | `vMAJOR.MINOR.PATCH` | `v3.0.0` | Production releases |
| Pre-release | `vX.Y.Z-beta.N` | `v3.0.0-beta.1` | Testing before stable |
| Release Candidate | `vX.Y.Z-rc.N` | `v3.0.0-rc.1` | Final testing |

### Creating Tags

```bash
# Stable release
git tag -a v3.0.0 -m "Release v3.0.0"

# Beta release
git tag -a v3.0.0-beta.1 -m "Release v3.0.0-beta.1"

# Release candidate
git tag -a v3.0.0-rc.1 -m "Release v3.0.0-rc.1"
```

### Listing Tags

```bash
# List all tags
git tag

# List tags sorted by version
git tag --sort=-v:refname

# List recent tags
git tag --sort=-v:refname | head -10
```

---

## 🚨 Troubleshooting

### Release Failed to Build

**Issue:** Goreleaser workflow fails

```bash
# View workflow logs
gh run list --workflow=goreleaser.yaml --limit 1
gh run view <run-id> --log-failed

# Test locally
task go:release-test

# Check .goreleaser.yaml syntax
goreleaser check
```

### Tag Already Exists

**Issue:** Tag already exists locally or remotely

```bash
# Delete local tag
git tag -d v3.0.0

# Delete remote tag (careful!)
git push origin :refs/tags/v3.0.0

# Or use the task
task go:release-delete TAG=v3.0.0
```

---

## 📊 Release Analytics

### View Release Downloads

```bash
# Via gh CLI
gh release view v3.0.0

# View all releases
gh release list

# Download statistics (requires jq)
gh api repos/CowDogMoo/warpgate/releases/latest | jq '.assets[] | {name, download_count}'
```

### Release Metrics

```bash
# Time between releases
git log --tags --simplify-by-decoration --pretty="format:%ai %d" | head -10

# Commits per release
git log v2.0.1..v3.0.0 --oneline | wc -l

# Contributors
git shortlog v2.0.1..v3.0.0 -sn
```

---

## 📚 Related Documentation

- [Commands Reference](./commands.md) - CLI documentation
- [Troubleshooting Guide](./troubleshooting.md) - Common issues
- [Semantic Versioning](https://semver.org/) - Version numbering standard
- [GoReleaser Documentation](https://goreleaser.com/) - Release tool docs
- [Conventional Commits](https://www.conventionalcommits.org/) -
  Commit message format
