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

DockerLogin authenticates with a Docker registry using the provided username
and token. It executes the 'docker login' command.

**Parameters:**

username: The username for the Docker registry.
token: The access token for the Docker registry.

**Returns:**

error: An error if any issue occurs during the login process.

---

### DockerManifestCreate(string, []string)

```go
DockerManifestCreate(string, []string) error
```

DockerManifestCreate creates a Docker manifest that references multiple
platform-specific versions of an image. It builds the manifest using the
'docker manifest create' command.

**Parameters:**

manifest: The name of the manifest to create.
images: A slice of image names to include in the manifest.

**Returns:**

error: An error if the manifest creation fails.

---

### DockerManifestPush(string)

```go
DockerManifestPush(string) error
```

DockerManifestPush pushes a Docker manifest to a registry. It uses the
'docker manifest push' command.

**Parameters:**

manifest: The name of the manifest to push.

**Returns:**

error: An error if the push operation fails.

---

### DockerPush(string)

```go
DockerPush(string) error
```

DockerPush pushes a Docker image to a registry. It executes the 'docker push'
command with the specified image name.

**Parameters:**

image: The name of the image to push.

**Returns:**

error: An error if the push operation fails.

---

### DockerTag(string)

```go
DockerTag(string) error
```

DockerTag tags a Docker image with a new name. It performs the operation
using the 'docker tag' command.

**Parameters:**

sourceImage: The current name of the image.
targetImage: The new name to assign to the image.

**Returns:**

error: An error if the tagging operation fails.

---

### ValidateToken(string)

```go
ValidateToken(string) error
```

ValidateToken checks the validity of a GitHub access token by making
a GET request to the GitHub API. It sets the Authorization header with
the token and examines the response status code.

**Parameters:**

token: The GitHub access token to validate.

**Returns:**

error: An error if the token is invalid, or if any issue occurs during
the request or reading the response.

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
