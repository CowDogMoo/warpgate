# WarpGate/packer

The `packer` package is a part of the WarpGate.

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

### BlueprintPacker.ParseImageHashes(string)

```go
ParseImageHashes(string)
```

ParseImageHashes extracts the image hashes from the output of a Packer build
command and updates the provided Packer blueprint with the new hashes.

**Parameters:**

output: The output from the Packer build command.

---

### BlueprintPacker.RunBuild([]string, string)

```go
RunBuild([]string, string) error
```

RunBuild runs the build command with the provided arguments.

**Parameters:**
- args: The arguments for the build command.

**Returns:**
- error: An error if the build command fails.

---

### BlueprintPacker.RunInit([]string, string)

```go
RunInit([]string, string) error
```

RunInit runs the init command with the provided arguments.

---

### BlueprintPacker.RunValidate([]string, string)

```go
RunValidate([]string, string) error
```

RunValidate runs the validate command with the provided arguments.

---

### BlueprintPacker.RunVersion()

```go
RunVersion() string, error
```

RunVersion runs the version command and returns the Packer version.

---

### LoadPackerTemplates()

```go
LoadPackerTemplates() []BlueprintPacker, error
```

LoadPackerTemplates loads Packer templates from the configuration file.

**Returns:**

[]BlueprintPacker: A slice of Packer templates.
error: An error if any issue occurs while loading the Packer templates.

---

## Installation

To use the WarpGate/packer package, you first need to install it.
Follow the steps below to install via go install.

```bash
go install github.com/cowdogmoo/warpgate/packer@latest
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/packer"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/packer`:

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
