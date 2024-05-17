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
DockerLogin(string) error
```

DockerLogin authenticates with a Docker registry using the provided
username, password, and server. It constructs an auth string for
the registry.

**Parameters:**

username: The username for the Docker registry.
password: The password for the Docker registry.
server: The server address of the Docker registry.

**Returns:**

string: The base64 encoded auth string.
error: An error if any issue occurs during the login process.

---

### DockerClient.DockerManifestCreate(string, []string)

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

### DockerClient.DockerManifestPush(string)

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

### DockerClient.DockerPush(string)

```go
DockerPush(string) error
```

DockerPush pushes a Docker image to a registry using the provided
auth string.

**Parameters:**

containerImage: The name of the image to push.
authStr: The auth string for the Docker registry.

**Returns:**

error: An error if the push operation fails.

---

### DockerClient.DockerTag(string)

```go
DockerTag(string) error
```

DockerTag tags a Docker image with a new name.

**Parameters:**

sourceImage: The current name of the image.
targetImage: The new name to assign to the image.

**Returns:**

error: An error if the tagging operation fails.

---

### DockerClient.TagAndPushImages([]packer.BlueprintPacker)

```go
TagAndPushImages([]packer.BlueprintPacker) error
```

TagAndPushImages tags and pushes images specified in packer templates.

**Parameters:**

packerTemplates: A slice of BlueprintPacker containing the images to tag
and push.

**Returns:**

error: An error if any operation fails during tagging or pushing.

---

### NewDockerClient()

```go
NewDockerClient() *DockerClient, error
```

NewDockerClient creates a new Docker client.

**Returns:**

*DockerClient: A DockerClient instance.
error: An error if any issue occurs while creating the client.

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
