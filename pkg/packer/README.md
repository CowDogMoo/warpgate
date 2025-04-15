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

### CheckRequiredEnvVars([]string)

```go
CheckRequiredEnvVars([]string) error
```

CheckRequiredEnvVars ensures that the necessary environment variables are set.

**Parameters:**

vars: A list of environment variable names to check.

**Returns:**

error: An error if any of the environment variables are not set.

---

### PackerTemplates.ParseAMIDetails(string)

```go
ParseAMIDetails(string) string
```

ParseAMIDetails extracts the AMI ID from the output of a Packer build command.

**Parameters:**

output: The output from the Packer build command.

**Returns:**

string: The AMI ID if found in the output.

---

### PackerTemplates.ParseImageHashes(string)

```go
ParseImageHashes(string) []ImageHash
```

ParseImageHashes extracts image hashes from Packer build output and updates
the provided PackerTemplates struct.

**Parameters:**

output: The output from the Packer build command.

**Returns:**

[]ImageHash: A slice of ImageHash structs parsed from the build output.

---

### PackerTemplates.RunBuild([]string, string)

```go
RunBuild([]string, string) []ImageHash, string, error
```

RunBuild runs the Packer build command and captures the output to parse image
hashes and AMI details.

**Parameters:**

args: A slice of strings containing the arguments to pass to the Packer build command.
dir: The directory to run the Packer build command in.

**Returns:**

[]ImageHash: A slice of image hashes parsed from the build output.
string: The AMI ID parsed from the build output.
error: An error if the build command fails.

---

### PackerTemplates.RunInit([]string, string)

```go
RunInit([]string, string) error
```

RunInit runs the Packer init command with the provided arguments.

**Parameters:**

args: A slice of strings representing the init command arguments.
dir: The directory in which to run the command. If empty, the current
directory is used.

**Returns:**

error: An error if the init command fails.

---

### PackerTemplates.RunValidate([]string, string)

```go
RunValidate([]string, string) error
```

RunValidate runs the Packer validate command with the provided arguments.

**Parameters:**

args: A slice of strings representing the validate command arguments.
dir: The directory in which to run the command. If empty, the current
directory is used.

**Returns:**

error: An error if the validate command fails.

---

### PackerTemplates.RunVersion()

```go
RunVersion() string, error
```

RunVersion runs the Packer version command and returns the Packer version.

**Returns:**

string: The version of Packer.
error: An error if the version command fails.

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
