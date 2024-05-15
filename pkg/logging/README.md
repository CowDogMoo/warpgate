# WarpGate/logging

The `logging` package is a part of the WarpGate.

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

### CustomLogger.Debug(string, ...interface{})

```go
Debug(string, ...interface{})
```

Debug logs a debug message.

**Parameters:**

format: The format string for the log message.
args: The arguments to be formatted into the log message.

---

### CustomLogger.Error(interface{}, ...interface{})

```go
Error(interface{}, ...interface{})
```

Error logs an error message.

**Parameters:**

firstArg: The first argument, which can be a string, an error, or any other type.
args: Additional arguments to be formatted into the log message.

---

### CustomLogger.Info(string, ...interface{})

```go
Info(string, ...interface{})
```

Info logs an informational message.

**Parameters:**

format: The format string for the log message.
args: The arguments to be formatted into the log message.

---

### CustomLogger.Warn(string, ...interface{})

```go
Warn(string, ...interface{})
```

Warn logs a warning message.

**Parameters:**

format: The format string for the log message.
args: The arguments to be formatted into the log message.

---

### Debug(string, ...interface{})

```go
Debug(string, ...interface{})
```

Debug logs a debug message using the global logger.

**Parameters:**

message: The format string for the log message.
args: The arguments to be formatted into the log message.

---

### Error(interface{}, ...interface{})

```go
Error(interface{}, ...interface{})
```

Error logs an error message using the global logger.

**Parameters:**

firstArg: The first argument, which can be a string, an error, or any other type.
args: Additional arguments to be formatted into the log message.

---

### Info(string, ...interface{})

```go
Info(string, ...interface{})
```

Info logs an informational message using the global logger.

**Parameters:**

message: The format string for the log message.
args: The arguments to be formatted into the log message.

---

### Initialize(string)

```go
Initialize(string) error
```

Initialize sets up the global logger.

**Parameters:**

configDir: The directory where the log file will be stored.
logLevelStr: The log level as a string from the configuration.

**Returns:**

error: An error if the logger initialization fails.

---

### NewCustomLogger(slog.Level)

```go
NewCustomLogger(slog.Level) *CustomLogger
```

NewCustomLogger creates a new instance of CustomLogger.

**Parameters:**

level: The logging level to be set for the logger.

**Returns:**

*CustomLogger: A pointer to the newly created CustomLogger instance.

---

### Warn(string, ...interface{})

```go
Warn(string, ...interface{})
```

Warn logs a warning message using the global logger.

**Parameters:**

message: The format string for the log message.
args: The arguments to be formatted into the log message.

---

## Installation

To use the WarpGate/logging package, you first need to install it.
Follow the steps below to install via go install.

```bash
go install github.com/cowdogmoo/warpgate/logging@latest
```

---

## Usage

After installation, you can import the package in your Go project
using the following import statement:

```go
import "github.com/cowdogmoo/warpgate/logging"
```

---

## Tests

To ensure the package is working correctly, run the following
command to execute the tests for `WarpGate/logging`:

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
