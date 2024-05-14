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
Follow the steps below to install via go get.

```bash
go get github.com/cowdogmoo/warpgate/packer
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
