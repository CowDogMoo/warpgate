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

### CustomAuthenticator.Authorization()

```go
Authorization() *authn.AuthConfig, error
```

Authorization returns the value to use in an http transport's Authorization header.

---

### DefaultGetStore(storage.StoreOptions)

```go
DefaultGetStore(storage.StoreOptions) storage.Store, error
```

DefaultGetStore returns a storage.Store instance with the provided
options.

**Parameters:**

options: Storage options for the store.

**Returns:**

storage.Store: A storage.Store instance.
error: An error if any issue occurs while getting the store.

---

### DockerClient.CreateAndPushManifest(*packer.PackerTemplates, []string)

```go
CreateAndPushManifest(*packer.PackerTemplates, []string) error
```

CreateAndPushManifest creates a manifest list and pushes it to a registry.

**Parameters:**

pTmpl: Packer templates containing the tag information.
imageTags: A slice of image tags to include in the manifest list.

**Returns:**

error: An error if any operation fails during manifest creation or pushing.

---

### DockerClient.CreateManifest(context.Context, string, []string, authn.Keychain)

```go
CreateManifest(context.Context string []string authn.Keychain) v1.ImageIndex error
```

CreateManifest creates a manifest list with the input image tags
and the specified target image.

**Parameters:**

ctx: The context within which the manifest list is created.
targetImage: The name of the image to create the manifest list for.
imageTags: A slice of image tags to include in the manifest list.
keychain: The keychain to use for authentication.

**Returns:**

v1.ImageIndex: The manifest list created with the input image tags.
error: An error if any operation fails during the manifest list creation.

---

### DockerClient.DockerLogin()

```go
DockerLogin() error
```

DockerLogin authenticates with a Docker registry using the provided
credentials.

**Returns:**

error: An error if the login operation fails.

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

### DockerClient.GetImageSize(string)

```go
GetImageSize(string) int64, error
```

GetImageSize returns the size of the image with the input reference.

**Parameters:**

imageRef: The reference of the image to get the size of.

**Returns:**

int64: The size of the image in bytes
error: An error if any operation fails during the size retrieval

---

### DockerClient.ProcessTemplates(*packer.PackerTemplates, string)

```go
ProcessTemplates(*packer.PackerTemplates, string) error
```

ProcessTemplates processes Packer templates by tagging and pushing images
to a registry.

**Parameters:**

pTmpl: A PackerTemplate containing the image to process.
blueprint: The blueprint containing tag information.

**Returns:**

error: An error if any operation fails during tagging or pushing.

---

### DockerClient.PushImage(string)

```go
PushImage(string) error
```

DockerPush pushes a Docker image to a registry using the provided
auth string.

**Parameters:**

containerImage: The name of the image to push.
authStr: The auth string for the Docker registry.

**Returns:**

error: An error if the push operation fails.

---

### DockerClient.PushManifest(string, v1.ImageIndex, authn.Keychain)

```go
PushManifest(string, v1.ImageIndex, authn.Keychain) error
```

PushManifest pushes the input manifest list to the registry.

**Parameters:**

imageName: The name of the image to push the manifest list for.
manifestList: The manifest list to push.
keychain: The keychain to use for authentication.

**Returns:**

error: An error if any operation fails during the push.

---

### DockerClient.RemoveImage(context.Context, string, image.RemoveOptions)

```go
RemoveImage(context.Context string image.RemoveOptions) []image.DeleteResponse error
```

RemoveImage removes an image from the Docker client.

**Parameters:**

ctx: The context within which the image is to be removed.
imageID: The ID of the image to be removed.
options: Options for the image removal operation.

**Returns:**

error: An error if any issue occurs during the image removal process.
[]image.DeleteResponse: A slice of image.DeleteResponse instances.

---

### DockerClient.SetRegistry(*DockerRegistry)

```go
SetRegistry(*DockerRegistry)
```

SetRegistry sets the DockerRegistry for the DockerClient.

**Parameters:**

registry: A pointer to the DockerRegistry to be set.

---

### DockerClient.TagAndPushImages(*packer.PackerTemplates)

```go
TagAndPushImages(*packer.PackerTemplates) []string, error
```

TagAndPushImages tags and pushes images to a registry based on
the provided PackerTemplate.

**Parameters:**

pTmpl: The PackerTemplates containing image tag information.

**Returns:**

[]string: A slice of image tags that were successfully pushed.
error: An error if any operation fails during tagging or pushing.

---

### NewDockerClient(packer.ContainerImageRegistry)

```go
NewDockerClient(packer.ContainerImageRegistry) *DockerClient, error
```

NewDockerClient creates a new Docker client.

**Returns:**

*DockerClient: A DockerClient instance.
error: An error if any issue occurs while creating the client.

---

### NewDockerRegistry(packer.ContainerImageRegistry, GetStoreFunc, bool)

```go
NewDockerRegistry(packer.ContainerImageRegistry GetStoreFunc bool) *DockerRegistry error
```

NewDockerRegistry creates a new Docker registry.

**Parameters:**

registryURL: The URL of the Docker registry.
authToken: The authentication token for the registry.
registryConfig: A packer.ContainerImageRegistry instance.
getStore: A function that returns a storage.Store instance.
ignoreChownErrors: A boolean indicating whether to ignore chown errors.

**Returns:**

*DockerRegistry: A DockerRegistry instance.
error: An error if any issue occurs while creating the registry.

---

### RemoteClient.Image(name.Reference, ...remote.Option)

```go
Image(name.Reference, ...remote.Option) v1.Image, error
```

Image retrieves a container image from a registry.

**Parameters:**

ref: The name.Reference instance for the image.
options: Options for the remote operation.

**Returns:**

v1.Image: The container image.
error: An error if the operation fails.

---

### customHelper.Get(string)

```go
Get(string) string, string, error
```

Get retrieves the username and password for the input registry.

**Parameters:**

registry: The registry to get the username and password for.

**Returns:**

string: The username for the registry.
string: The password for the registry.
error: An error if any operation fails during the retrieval.

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
