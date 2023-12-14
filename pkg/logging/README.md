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

### ColoredLogger.Debug(...interface{})

```go
Debug(...interface{})
```

Debug logs a debug message in the specified color.

---

### ColoredLogger.Debugf(string, ...interface{})

```go
Debugf(string, ...interface{})
```

Debugf logs a formatted debug message in the specified color.

---

### ColoredLogger.Error(...interface{})

```go
Error(...interface{})
```

Error logs an error message in bold and the specified color.

---

### ColoredLogger.Errorf(string, ...interface{})

```go
Errorf(string, ...interface{})
```

Errorf logs a formatted error message in bold and the specified color.

---

### ColoredLogger.Printf(string, ...interface{})

```go
Printf(string, ...interface{})
```

Printf logs a formatted string in the specified color.

---

### ColoredLogger.Println(...interface{})

```go
Println(...interface{})
```

Println logs a line with the provided arguments in the specified color.

---

### CreateLogFile(afero.Fs, string, string)

```go
CreateLogFile(afero.Fs, string, string) LogInfo, error
```

CreateLogFile creates a log file in a specified directory. It ensures
the directory exists and creates a new log file if it doesn't exist.

**Parameters:**

fs: Filesystem interface for file operations.
logDir: Directory to create the log file in.
logName: Name of the log file to create.

**Returns:**

LogInfo: Information about the created log file.
error: An error if there is a failure in creating the log file.

---

### GetLogLevel(string)

```go
GetLogLevel(string) slog.Level
```

GetLogLevel determines the slog.Level based on a provided string.
It supports 'debug' and 'info' levels, defaulting to 'info' if
the input does not match these values.

**Parameters:**

level: A string representing the desired log level.

**Returns:**

slog.Level: The corresponding slog log level for the provided string.

---

### InitGlobalLogger(slog.Level, string)

```go
InitGlobalLogger(slog.Level, string) error
```

InitGlobalLogger initializes the global logger with the specified level and file path.
This function should be called at the beginning of your application.

---

### L()

```go
L() Logger
```

L returns the global logger instance.

---

### PlainLogger.Debug(...interface{})

```go
Debug(...interface{})
```

Debug logs a debug message in plain text format.

---

### PlainLogger.Debugf(string, ...interface{})

```go
Debugf(string, ...interface{})
```

Debugf logs a formatted debug message in plain text format.

---

### PlainLogger.Error(...interface{})

```go
Error(...interface{})
```

Error logs an error message in plain text format.

---

### PlainLogger.Errorf(string, ...interface{})

```go
Errorf(string, ...interface{})
```

Errorf logs a formatted error message in plain text format.

---

### PlainLogger.Printf(string, ...interface{})

```go
Printf(string, ...interface{})
```

Printf logs a formatted string in plain text format.

---

### PlainLogger.Println(...interface{})

```go
Println(...interface{})
```

Println logs a line with the provided arguments in plain text format.

---

### SlogLogger.Debug(...interface{})

```go
Debug(...interface{})
```

Debug logs a debug message using slog library.

---

### SlogLogger.Debugf(string, ...interface{})

```go
Debugf(string, ...interface{})
```

Debugf logs a formatted debug message using slog library.

---

### SlogLogger.Error(...interface{})

```go
Error(...interface{})
```

Error logs an error message using slog library.

---

### SlogLogger.Errorf(string, ...interface{})

```go
Errorf(string, ...interface{})
```

Errorf logs a formatted error message using slog library.

---

### SlogLogger.Printf(string, ...interface{})

```go
Printf(string, ...interface{})
```

Printf logs a formatted string using slog library.

---

### SlogLogger.Println(...interface{})

```go
Println(...interface{})
```

Println logs a line with the provided arguments using slog library.

---

### SlogPlainLogger.Debug(...interface{})

```go
Debug(...interface{})
```

Debug logs a debug message using slog library.

---

### SlogPlainLogger.Debugf(string, ...interface{})

```go
Debugf(string, ...interface{})
```

Debugf logs a formatted debug message using slog library.

---

### SlogPlainLogger.Error(...interface{})

```go
Error(...interface{})
```

Error logs an error message using slog library.

---

### SlogPlainLogger.Errorf(string, ...interface{})

```go
Errorf(string, ...interface{})
```

Errorf logs a formatted error message using slog library.

---

### SlogPlainLogger.Printf(string, ...interface{})

```go
Printf(string, ...interface{})
```

Printf logs a formatted string using slog library.

---

### SlogPlainLogger.Println(...interface{})

```go
Println(...interface{})
```

Println logs a line with the provided arguments using slog library.

---

## Installation

To use the WarpGate/logging package, you first need to install it.
Follow the steps below to install via go get.

```bash
go get github.com/cowdogmoo/warpgate/logging
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
