# WarpGate/registry

The `registry` package is a part of the WarpGate.

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

### ValidateToken(string)

```go
ValidateToken(string) error
```

ValidateToken checks the validity of a GitHub access token by making
a GET request to the GitHub API. If no token is provided as an argument,
it checks for a GITHUB_TOKEN environment variable and uses that.
It sets the Authorization header with the token and examines the response status code.

**Parameters:**

token: The GitHub access token to validate. If empty, the function will
check for a GITHUB_TOKEN environment variable.

**Returns:**

error: An error if the token is invalid, or if any issue occurs during
the request or reading the response, or if no token is provided and
the GITHUB_TOKEN environment variable is not set.

---

## Installation

To use the WarpGate/registry package, you first need to install it.
Follow the steps below to install via go get.

```bash
go get github.com/cowdogmoo/warpgate/registry
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/registry"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/registry`:

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
