# WarpGate/magehelpers

The `magehelpers` package is a part of the WarpGate.

---

## Table of contents

- [Functions](#functions)
- [Installation](#installation)
- [Usage](#usage)
- [Tests](#tests)
- [Contributing](#contributing)
- [License](#license)

---

## Functions

### Compile()

```go
Compile() error
```

Compile compiles the Go project using goreleaser. The behavior is
controlled by the 'release' environment variable. If the GOOS and
GOARCH environment variables are not set, the function defaults
to the current system's OS and architecture.

**Environment Variables:**

release: Determines the compilation mode.

If "true", compiles all supported releases for warpgate.
If "false", compiles only the binary for the specified OS
and architecture (based on GOOS and GOARCH) or the current
system's default if the vars aren't set.

GOOS: Target operating system for compilation. Defaults to the
current system's OS if not set.

GOARCH: Target architecture for compilation. Defaults to the
current system's architecture if not set.

Example usage:

```go
release=true mage compile # Compiles all supported releases for warpgate
GOOS=darwin GOARCH=arm64 mage compile false # Compiles the binary for darwin/arm64
GOOS=linux GOARCH=amd64 mage compile false # Compiles the binary for linux/amd64
```

**Returns:**

error: An error if any issue occurs during compilation.

---

### GeneratePackageDocs()

```go
GeneratePackageDocs() error
```

GeneratePackageDocs creates documentation for the various packages
in the project.

Example usage:

```go
mage generatepackagedocs
```

**Returns:**

error: An error if any issue occurs during documentation generation.

---

### RunTests()

```go
RunTests() error
```

RunTests executes all unit tests.

Example usage:

```go
mage runtests
```

**Returns:**

error: An error if any issue occurs while running the tests.

---

## Installation

To use the WarpGate/magehelpers package, you first need to install it.
Follow the steps below to install via go install.

```bash
go install github.com/cowdogmoo/warpgate/magehelpers@latest
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/magehelpers"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/magehelpers`:

```bash
go test -v
```

---

## Contributing

Pull requests are welcome. For major changes,
please open an issue first to discuss what
you would like to change.

---

## License

This project is licensed under the MIT
License - see the [LICENSE](https://github.com/CowDogMoo/WarpGate/blob/main/LICENSE)
file for details.
