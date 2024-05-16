# WarpGate/docker

The `docker` package is a part of the WarpGate.

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

### DockerClient.DockerLogin(string)

```go
DockerLogin(string) string, error
```


---

### DockerClient.DockerPush(string)

```go
DockerPush(string) error
```


---

### DockerClient.DockerTag(string)

```go
DockerTag(string) error
```


---

### DockerClient.PushDockerImages([]packer.BlueprintPacker)

```go
PushDockerImages([]packer.BlueprintPacker) error
```


---

### NewDockerClient()

```go
NewDockerClient() *DockerClient, error
```


---

## Installation

To use the WarpGate/docker package, you first need to install it.
Follow the steps below to install via go install.

```bash
go install github.com/cowdogmoo/warpgate/docker@latest
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/docker"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/docker`:

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
