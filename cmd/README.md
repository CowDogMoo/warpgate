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

---

### RunImageBuilder(*cobra.Command, []string, bp.Blueprint)

```go
RunImageBuilder(*cobra.Command, []string, bp.Blueprint) error
```

RunImageBuilder is the main function for the imageBuilder command
that builds container images using Packer.

**Parameters:**

cmd: A Cobra command object containing flags and arguments for the command.
args: A slice of strings containing additional arguments passed to the command.
blueprint: A Blueprint struct containing the blueprint configuration.

**Returns:**

error: An error if any issue occurs while building the images.

---

## Installation

To use the WarpGate/cmd package, you first need to install it.
Follow the steps below to install via go install.

```bash
go install github.com/cowdogmoo/warpgate/cmd@latest
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
