# WarpGate/blueprint

The `blueprint` package is a part of the WarpGate.

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

### Blueprint.ParseCommandLineFlags(*cobra.Command)

```go
ParseCommandLineFlags(*cobra.Command) error
```

ParseCommandLineFlags parses command line flags for a Blueprint.

**Parameters:**

cmd: A Cobra command object containing flags and arguments for the command.

**Returns:**

error: An error if any issue occurs while parsing the command line flags.

---

### Blueprint.SetConfigPath()

```go
SetConfigPath() error
```

SetConfigPath sets the configuration path for a Blueprint.

**Returns:**

error: An error if the configuration path cannot be set.

---

## Installation

To use the WarpGate/blueprint package, you first need to install it.
Follow the steps below to install via go get.

```bash
go get github.com/cowdogmoo/warpgate/blueprint
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/blueprint"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/blueprint`:

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
