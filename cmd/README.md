# WarpGate/cmd

The `cmd` package is a part of the WarpGate.

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

### Execute()

```go
Execute()
```

Execute runs the root cobra command. It checks for errors and exits
the program if any are encountered.

**Returns:**

error: An error if the command execution fails.

---

### RunImageBuilder(*cobra.Command, []string)

```go
RunImageBuilder(*cobra.Command, []string) error
```

RunImageBuilder is the main function for the imageBuilder command
that builds container images using Packer.

**Parameters:**

cmd: A Cobra command object containing flags and arguments for the command.
args: A slice of strings containing additional arguments passed to the command.

**Returns:**

error: An error if any issue occurs while building the images.

---

### SetBlueprintConfigPath(string)

```go
SetBlueprintConfigPath(string) error
```

SetBlueprintConfigPath sets the configuration path for the blueprint.

**Parameters:**

blueprintDir: The directory where the blueprint configuration file is located.

**Returns:**

error: An error if any issue occurs while setting the configuration path.

---

## Installation

To use the WarpGate/cmd package, you first need to install it.
Follow the steps below to install via go get.

```bash
go get github.com/cowdogmoo/warpgate/cmd
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/cmd"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/cmd`:

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
