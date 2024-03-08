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

### DockerLogin(string)

```go
DockerLogin(string) error
```

DockerLogin logs in to the Docker registry.

---

### DockerManifestCreate(string, []string)

```go
DockerManifestCreate(string, []string) error
```

DockerManifestCreate creates a Docker manifest.

---

### DockerManifestPush(string)

```go
DockerManifestPush(string) error
```

DockerManifestPush pushes a Docker manifest to the registry.

---

### DockerPush(string)

```go
DockerPush(string) error
```

DockerPush pushes a Docker image to the registry.

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
