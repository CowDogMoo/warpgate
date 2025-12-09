# Contributing to Warpgate

Thank you for your interest in contributing to Warpgate! This document
provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Ways to Contribute](#ways-to-contribute)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)
- [Getting Help](#getting-help)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all
contributors. By participating in this project, you agree to:

- Be respectful and considerate of others
- Accept constructive criticism gracefully
- Focus on what is best for the community
- Show empathy towards other community members

If you witness or experience unacceptable behavior, please report it by
opening an issue or contacting the maintainers directly.

## Ways to Contribute

There are many ways to contribute to Warpgate:

### Report Bugs

Found a bug? Please
[open an issue](https://github.com/CowDogMoo/warpgate/issues/new) with:

- A clear, descriptive title
- Detailed description of the issue
- Steps to reproduce the behavior
- Expected vs actual behavior
- Your environment (OS, Go version, warpgate version)
- Relevant logs or error messages

**Example:**

```text
Title: Build fails with "storage driver" error on Ubuntu 22.04

Description:
When running `warpgate build attack-box`, the build fails with:
"Error: storage driver 'overlay' not supported"

Steps to Reproduce:
1. Install warpgate via `go install`
2. Run `warpgate build attack-box`
3. Observe error

Environment:
- OS: Ubuntu 22.04 LTS
- Go: 1.21.3
- Warpgate: v1.2.0
- Docker: 24.0.5

Expected: Build should succeed
Actual: Build fails with storage driver error
```

### Request Features

Have an idea for a new feature?
[Open a feature request](https://github.com/CowDogMoo/warpgate/issues/new)
with:

- Clear description of the feature
- Use cases and benefits
- Proposed implementation approach (optional)
- Examples from similar tools (if applicable)

### Improve Documentation

Documentation improvements are always welcome:

- Fix typos, grammar, or unclear explanations
- Add missing documentation
- Create tutorials or guides
- Improve code examples
- Update outdated information

Submit documentation changes through pull requests just like code changes.

### Submit Code

Fix bugs, implement features, or improve existing code:

1. Check existing [issues](https://github.com/CowDogMoo/warpgate/issues) and
   [pull requests](https://github.com/CowDogMoo/warpgate/pulls) to avoid
   duplication
2. For significant changes, open an issue first to discuss your approach
3. Follow the development workflow below

### Share Templates

Created a useful Warpgate template? Share it:

- Add it to the
  [warpgate-templates](https://github.com/cowdogmoo/warpgate-templates)
  repository
- Blog about your use case and template

## Getting Started

### Prerequisites

- **Go 1.21+** - [Install Go](https://go.dev/doc/install)
- **Git** - For version control
- **GitHub CLI (optional)** - `gh` tool for easier workflows
- **Task (optional)** - [Install Task](https://taskfile.dev/installation/)
  (`brew install go-task`)
- **Pre-commit (optional)** - For commit hooks (`brew install pre-commit`)

### Fork and Clone

```bash
# Fork the repository via GitHub web interface or CLI
gh repo fork CowDogMoo/warpgate --clone

# Navigate to repository
cd warpgate

# Add upstream remote
git remote add upstream https://github.com/CowDogMoo/warpgate.git
```

### Install Dependencies

```bash
# Download Go modules
go mod download

# Install development tools (optional)
task go:install-tools

# Install pre-commit hooks (recommended)
pre-commit install
```

### Verify Setup

```bash
# Build warpgate
task go:build

# Run tests
task go:test

# Run linters
task go:lint
```

If all commands succeed, you're ready to start contributing!

## Development Workflow

### 1. Create a Branch

Create a descriptive branch name using one of these prefixes:

- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Adding or updating tests
- `chore/` - Maintenance tasks

```bash
# Create and switch to new branch
git checkout -b feature/add-azure-ami-support

# Keep your branch up to date with main
git fetch upstream
git rebase upstream/main
```

### 2. Make Changes

Write your code following our [Code Standards](#code-standards):

```bash
# Edit files
vim pkg/builder/azure.go

# Run tests frequently
task go:test

# Format code
task go:fmt

# Check linting
task go:lint
```

### 3. Test Your Changes

```bash
# Run all tests
task go:test

# Run specific package tests
go test -v ./pkg/builder/

# Run with coverage
task go:test-coverage

# Test integration scenarios manually
./warpgate build test-template --var DEBUG=true
```

### 4. Commit Changes

We use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages:

**Format:**

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types:**

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `style:` - Code style changes (formatting, no logic change)
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks
- `ci:` - CI/CD changes

**Examples:**

```bash
# Feature
git commit -m "feat(builder): add Azure AMI support"

# Bug fix
git commit -m "fix(template): resolve variable substitution in ansible provisioner"

# Documentation
git commit -m "docs: add troubleshooting guide for storage errors"

# With body
git commit -m "feat(manifest): add multi-arch manifest support

Implements creation and pushing of multi-architecture container
image manifests for amd64 and arm64 platforms.

Closes #123"
```

### 5. Push and Create PR

```bash
# Push branch to your fork
git push origin feature/add-azure-ami-support

# Create pull request
gh pr create --title "feat(builder): add Azure AMI support" \
  --body "Adds support for building Azure VM images

Changes:
- Implements Azure Image Builder integration
- Adds Azure authentication support
- Updates documentation

Closes #123"
```

Or create the PR via GitHub web interface.

## Code Standards

### Go Style Guidelines

Follow standard Go conventions and idioms:

- **Use `gofmt`** - Format all code with `gofmt` (or `go fmt`)
- **Follow [Effective Go](https://go.dev/doc/effective_go)** - Standard Go best practices
- **Use [golangci-lint](https://golangci-lint.run/)** - Catch common issues
- **Write clear names** - Use descriptive variable and function names
- **Keep functions focused** - Each function should do one thing well
- **Document exported functions** - Add godoc comments for all exported symbols

**Example:**

```go
// BuildContainer builds a container image from the provided template.
// It returns the image ID and any error encountered during the build.
func BuildContainer(ctx context.Context, template *Template) (string, error) {
    if err := template.Validate(); err != nil {
        return "", fmt.Errorf("invalid template: %w", err)
    }

    builder, err := buildah.NewBuilder(ctx, template.BaseImage)
    if err != nil {
        return "", fmt.Errorf("failed to create builder: %w", err)
    }
    defer builder.Delete()

    // Build logic...

    return imageID, nil
}
```

### Error Handling

- **Wrap errors** - Add context with `fmt.Errorf("context: %w", err)`
- **Check all errors** - Never ignore errors
- **Return early** - Use early returns for error cases
- **Log appropriately** - Use appropriate log levels

**Example:**

```go
// Good: Wrapped errors with context
if err := builder.Run(command); err != nil {
    return fmt.Errorf("failed to run provisioner command %q: %w", command, err)
}

// Bad: Lost error context
if err := builder.Run(command); err != nil {
    return err
}
```

### Testing

- **Write table-driven tests** - For multiple test cases
- **Use descriptive test names** - Test names should describe what's being tested
- **Test edge cases** - Include happy path and error cases
- **Mock external dependencies** - Use interfaces and mocks
- **Aim for >80% coverage** - But prioritize meaningful tests

**Example:**

```go
func TestTemplateValidation(t *testing.T) {
    tests := []struct {
        name    string
        template *Template
        wantErr bool
    }{
        {
            name: "valid template",
            template: &Template{
                Name: "test",
                BaseImage: "ubuntu:22.04",
                Provisioners: []Provisioner{
                    {Type: "shell", Inline: []string{"echo test"}},
                },
            },
            wantErr: false,
        },
        {
            name: "missing base image",
            template: &Template{
                Name: "test",
                Provisioners: []Provisioner{
                    {Type: "shell", Inline: []string{"echo test"}},
                },
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.template.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation Requirements

- **Add godoc comments** - For all exported types, functions, and constants
- **Update README** - For user-facing changes
- **Add examples** - In code comments or docs/
- **Keep docs in sync** - Update docs when changing behavior

**Godoc example:**

```go
// Template represents a Warpgate build template configuration.
// It defines the base image, provisioners, and build targets for
// creating container images or cloud machine images.
type Template struct {
    // Name is the human-readable name of the template
    Name string `yaml:"name"`

    // BaseImage is the source image to build from (e.g., "ubuntu:22.04")
    BaseImage string `yaml:"base_image"`

    // Provisioners are executed in order to configure the image
    Provisioners []Provisioner `yaml:"provisioners"`
}
```

## Testing Guidelines

### Running Tests

```bash
# Run all tests
task go:test

# Run specific package
go test -v ./pkg/builder/

# Run with race detector
go test -race ./...

# Run with coverage
task go:test-coverage

# Run integration tests
go test -v -tags=integration ./...
```

### Writing Tests

**Unit Tests:**

- Test individual functions and methods
- Mock external dependencies
- Focus on business logic
- Use table-driven tests for multiple scenarios

**Integration Tests:**

- Test component interactions
- Use `// +build integration` build tag
- May require Docker or other dependencies
- Longer running, fewer of these

**Example unit test:**

```go
func TestBuilder_Build(t *testing.T) {
    ctx := context.Background()

    builder := &Builder{
        storage: &mockStorage{},
        runtime: &mockRuntime{},
    }

    template := &Template{
        Name: "test",
        BaseImage: "ubuntu:22.04",
    }

    imageID, err := builder.Build(ctx, template)
    if err != nil {
        t.Fatalf("Build() failed: %v", err)
    }

    if imageID == "" {
        t.Error("Build() returned empty image ID")
    }
}
```

**Example integration test:**

```go
// +build integration

func TestBuildRealImage(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Test with real Docker/Buildah
    ctx := context.Background()
    builder, err := NewBuilder(ctx)
    if err != nil {
        t.Fatalf("Failed to create builder: %v", err)
    }

    // Run real build...
}
```

### Test Coverage

- Aim for >80% code coverage
- Focus on critical paths
- Don't sacrifice test quality for coverage percentage
- Use `task go:test-coverage` to view coverage report

## Documentation

### Code Documentation

- **Godoc comments** - For all exported symbols
- **Examples** - Add example functions where helpful
- **Inline comments** - Explain complex logic

### User Documentation

Update user-facing documentation for:

- New features
- Changed behavior
- New configuration options
- Breaking changes

**Files to update:**

- `README.md` - Main documentation
- `docs/*.md` - Detailed guides
- Command help text - In cobra command files
- Template examples - In templates/ directory

### Writing Style

- Use clear, simple language
- Write in active voice
- Provide examples
- Be precise and accurate
- Keep it concise

## Pull Request Process

### Before Submitting

Checklist before opening a PR:

- [ ] Code follows Go conventions and style guidelines
- [ ] All tests pass (`task go:test`)
- [ ] Linters pass (`task go:lint`)
- [ ] Pre-commit hooks pass (`task pre-commit:run`)
- [ ] Added tests for new functionality
- [ ] Updated documentation for user-facing changes
- [ ] Commits follow conventional commit format
- [ ] Branch is up to date with main

### PR Description

Include in your PR description:

1. **Summary** - What does this PR do?
2. **Motivation** - Why is this change needed?
3. **Changes** - What specifically changed?
4. **Testing** - How was this tested?
5. **Screenshots** - If applicable (for UI/CLI output changes)
6. **Breaking Changes** - If any, clearly document them
7. **Related Issues** - Link related issues with "Closes #123"

**Example PR description:**

```markdown
## Summary

Adds support for building Azure VM images using Azure Image Builder.

## Motivation

Users have requested Azure support in addition to AWS AMIs. This enables
teams using Azure to build standardized VM images with Warpgate.

## Changes

- Implements Azure Image Builder integration in `pkg/builder/azure.go`
- Adds Azure authentication using Azure SDK
- Adds `--target azure` CLI flag
- Updates configuration to support Azure settings
- Adds integration tests for Azure builds

## Testing

- Added unit tests for Azure builder
- Tested manually with real Azure subscription
- Verified existing AWS functionality still works

## Breaking Changes

None - this is purely additive.

## Related Issues

Closes #123
Relates to #456
```

### Review Process

1. **Automated checks** - CI runs tests, linters, security scans
2. **Maintainer review** - A maintainer will review your code
3. **Feedback** - Address any requested changes
4. **Approval** - Once approved, your PR will be merged

### Responding to Feedback

- Be receptive to feedback
- Ask questions if unclear
- Make requested changes promptly
- Push new commits to the same branch
- Request re-review after addressing feedback

## Release Process

Releases are handled by maintainers following this process:

### Version Numbers

We follow [Semantic Versioning](https://semver.org/):

- **Major** (x.0.0) - Breaking changes
- **Minor** (0.x.0) - New features, backward compatible
- **Patch** (0.0.x) - Bug fixes, backward compatible

### Creating a Release

Maintainers use this process:

```bash
# Ensure main is up to date
git checkout main
git pull upstream main

# Run full test suite
task go:test
task go:lint
task pre-commit:run

# Create and push tag
task go:release TAG=v1.2.3

# Or manually:
git tag -a v1.2.3 -m "Release v1.2.3"
git push upstream v1.2.3
```

GitHub Actions will:

1. Run all tests and checks
2. Build binaries for all platforms
3. Create GitHub release with artifacts
4. Build and push Docker images

### Changelog

Update CHANGELOG.md before each release with:

- New features
- Bug fixes
- Breaking changes
- Deprecated features
- Security updates

## Getting Help

### Questions and Discussions

- **Bug reports** - Open an [Issue](https://github.com/CowDogMoo/warpgate/issues)
- **Feature requests** - Open an [Issue](https://github.com/CowDogMoo/warpgate/issues)
- **Security issues** - Email security@cowdogmoo.com (do not open public issues)

### Resources

- [README](README.md) - Project overview and usage
- [Template Configuration Guide](docs/template-configuration.md) - Template management
- [Go Documentation](https://pkg.go.dev/github.com/cowdogmoo/warpgate) -
  Code documentation

### Contact Maintainers

- GitHub: [@l50](https://github.com/l50)
- Project: [CowDogMoo](https://github.com/CowDogMoo)

## License

By contributing to Warpgate, you agree that your contributions will be
licensed under the [MIT License](LICENSE).

---

**Thank you for contributing to Warpgate!**

Your contributions help make infrastructure image building easier for everyone.
