# WarpGate/cloudstorage

The `cloudstorage` package is a part of the WarpGate.

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

### CleanupBucket(*CloudStorage)

```go
CleanupBucket(*CloudStorage) error
```

CleanupBucket destroys the S3 bucket created for the blueprint.

**Returns:**

error: An error if the S3 bucket cleanup fails.

---

### InitializeS3Bucket(*CloudStorage)

```go
InitializeS3Bucket(*CloudStorage) error
```

InitializeS3Bucket initializes an S3 bucket and stores the bucket name.

**Returns:**

error: An error if the S3 bucket initialization fails.

---

## Installation

To use the WarpGate/cloudstorage package, you first need to install it.
Follow the steps below to install via go install.

```bash
go install github.com/cowdogmoo/warpgate/cloudstorage@latest
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/cloudstorage"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/cloudstorage`:

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
