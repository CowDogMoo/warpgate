/*
Copyright © 2025 Jayson Grace <jayson.e.grace@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

// Package logging provides a custom logger with support for multiple output formats and log levels.
// All logging should be done through context-based functions (InfoContext, WarnContext, etc.)
// to ensure proper logger propagation through the application.
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
)

// LogLevel represents the severity level of a log message
type LogLevel int

// OutputType represents the output format for logs
type OutputType int

// Output types for different log formats
const (
	PlainOutput OutputType = iota
	ColorOutput
	JSONOutput
)

// Log levels for different types of log messages.
// Ordered from least to most severe for numeric comparison.
const (
	// DebugLevel represents debug messages (lowest severity)
	DebugLevel LogLevel = iota
	// InfoLevel represents informational messages
	InfoLevel
	// WarnLevel represents warning messages
	WarnLevel
	// ErrorLevel represents error messages (highest severity)
	ErrorLevel
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "INFO"
	}
}

// CustomLogger wraps the logging functionality with custom formatting options.
type CustomLogger struct {
	mu            sync.Mutex
	LogLevel      slog.Level
	OutputType    OutputType
	Quiet         bool
	ConsoleWriter io.Writer
	Verbose       bool
}

// formatMessage handles formatting based on output type and log level.
// For ColorOutput, it includes a colored level prefix.
// For PlainOutput, it returns the message as-is.
func (l *CustomLogger) formatMessage(level LogLevel, message string, args ...interface{}) string {
	formattedMsg := fmt.Sprintf(message, args...)

	if l.OutputType != ColorOutput {
		return formattedMsg
	}

	// Apply colored level prefix for color output
	switch level {
	case DebugLevel:
		return color.HiBlackString("[DEBUG] %s", formattedMsg)
	case InfoLevel:
		return color.HiGreenString("[INFO] %s", formattedMsg)
	case WarnLevel:
		return color.HiYellowString("[WARN] %s", formattedMsg)
	case ErrorLevel:
		return color.HiRedString("[ERROR] %s", formattedMsg)
	default:
		return formattedMsg
	}
}

// shouldShowOnConsoleLocked determines if a message should be shown on console.
// This method must be called while holding l.mu.
// Logic:
// - In quiet mode, only errors are shown
// - In verbose mode, all messages are shown
// - Otherwise, show messages at INFO level and above (INFO, WARN, ERROR)
func (l *CustomLogger) shouldShowOnConsoleLocked(level LogLevel) bool {
	// In quiet mode, only show errors
	if l.Quiet {
		return level == ErrorLevel
	}

	// In verbose mode, show all messages
	if l.Verbose {
		return true
	}

	// Default: show INFO and above (INFO, WARN, ERROR but not DEBUG)
	return level >= InfoLevel
}

func (l *CustomLogger) log(level LogLevel, message string, args ...interface{}) {
	formattedMsg := l.formatMessage(level, message, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	l.mu.Lock()
	shouldShow := l.shouldShowOnConsoleLocked(level)
	writer := l.ConsoleWriter
	l.mu.Unlock()

	if shouldShow && writer != nil {
		l.mu.Lock()
		if _, err := fmt.Fprintf(writer, "[%s] %s\n", timestamp, formattedMsg); err != nil {
			// Fallback to stderr if ConsoleWriter fails
			fmt.Fprintf(os.Stderr, "[%s] %s\n", timestamp, formattedMsg)
		}
		l.mu.Unlock()
	}
}

// NewCustomLogger creates a new instance of CustomLogger.
func NewCustomLogger(level slog.Level) *CustomLogger {
	return &CustomLogger{
		LogLevel:      level,
		Quiet:         false,
		ConsoleWriter: os.Stderr, // Default to stderr for CLI output
		Verbose:       false,
		OutputType:    PlainOutput,
	}
}

// NewCustomLoggerWithOptions creates a new CustomLogger with full configuration.
func NewCustomLoggerWithOptions(logLevelStr, outputFormat string, quiet, verbose bool) *CustomLogger {
	logLevel := DetermineLogLevel(logLevelStr)

	// Map the format string to the appropriate OutputType
	outputType := PlainOutput
	switch outputFormat {
	case "json":
		outputType = JSONOutput
	case "color":
		outputType = ColorOutput
	case "text", "plain":
		outputType = PlainOutput
	}

	// If verbose is set, ensure we're at least at debug level
	if verbose {
		if logLevel > slog.LevelDebug {
			logLevel = slog.LevelDebug
		}
	}

	return &CustomLogger{
		LogLevel:      logLevel,
		OutputType:    outputType,
		Quiet:         quiet,
		ConsoleWriter: os.Stderr,
		Verbose:       verbose,
	}
}

// SetQuiet enables or disables quiet mode.
// In quiet mode, only error messages are displayed.
// This method is thread-safe.
func (l *CustomLogger) SetQuiet(quiet bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Quiet = quiet
}

// SetVerbose enables or disables verbose mode.
// In verbose mode, info and debug messages are displayed on console.
// This method is thread-safe.
func (l *CustomLogger) SetVerbose(verbose bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Verbose = verbose
}

// IsQuiet returns whether the logger is in quiet mode.
// This method is thread-safe.
func (l *CustomLogger) IsQuiet() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Quiet
}

// Info logs an informational message.
func (l *CustomLogger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Output sends data to stdout.
func (l *CustomLogger) Output(data interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch l.OutputType {
	case JSONOutput:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			l.Error("Failed to encode JSON output: %v", err)
		}
	default:
		if _, err := fmt.Fprintln(os.Stdout, data); err != nil {
			l.Error("Failed to write output: %v", err)
		}
	}
}

// Print writes raw output to stdout without adding a newline.
// Use this for streaming output that already contains newlines.
func (l *CustomLogger) Print(data string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := fmt.Fprint(os.Stdout, data); err != nil {
		l.Error("Failed to write output: %v", err)
	}
}

// Warn logs a warning message.
func (l *CustomLogger) Warn(format string, args ...interface{}) {
	l.log(WarnLevel, format, args...)
}

// Debug logs a debug message.
func (l *CustomLogger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Error logs an error message. It accepts either an error, a format string,
// or any other value as the first argument.
func (l *CustomLogger) Error(firstArg interface{}, args ...interface{}) {
	switch v := firstArg.(type) {
	case error:
		if len(args) == 0 {
			l.log(ErrorLevel, "%s", v.Error())
		} else {
			l.log(ErrorLevel, v.Error(), args...)
		}
	case string:
		l.log(ErrorLevel, v, args...)
	default:
		l.log(ErrorLevel, "%v", v)
	}
}

// Errorf logs a formatted error message with type-safe format string.
func (l *CustomLogger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, format, args...)
}

// ErrorErr logs an error value directly without formatting.
func (l *CustomLogger) ErrorErr(err error) {
	if err != nil {
		l.log(ErrorLevel, "%s", err.Error())
	}
}

// DetermineLogLevel converts a string to slog.Level
func DetermineLogLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Context-based logging support

// loggerKeyType is the type for the logger context key
type loggerKeyType struct{}

// loggerKey is the context key for storing the logger
var loggerKey = loggerKeyType{}

// WithLogger returns a new context with the provided logger.
func WithLogger(ctx context.Context, l *CustomLogger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves the logger from the context.
// If no logger is found in context, returns a new default logger instance.
func FromContext(ctx context.Context) *CustomLogger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey).(*CustomLogger); ok && l != nil {
			return l
		}
	}

	// Return a new default logger - no global state, no race conditions
	return NewCustomLogger(slog.LevelInfo)
}

// Context-aware logging functions
// These functions retrieve the logger from context and use it.
// If no logger is found in context, a default logger is used.

// InfoContext logs an informational message using the logger from context.
func InfoContext(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).Info(message, args...)
}

// WarnContext logs a warning message using the logger from context.
func WarnContext(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).Warn(message, args...)
}

// DebugContext logs a debug message using the logger from context.
func DebugContext(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).Debug(message, args...)
}

// ErrorContext logs an error message using the logger from context. It accepts either
// an error, a format string, or any other value as the first argument.
func ErrorContext(ctx context.Context, firstArg interface{}, args ...interface{}) {
	FromContext(ctx).Error(firstArg, args...)
}

// ErrorfContext logs a formatted error message using the logger from context.
func ErrorfContext(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Errorf(format, args...)
}

// ErrorErrContext logs an error value using the logger from context.
func ErrorErrContext(ctx context.Context, err error) {
	FromContext(ctx).ErrorErr(err)
}

// OutputContext sends data to stdout using the logger from context.
func OutputContext(ctx context.Context, data interface{}) {
	FromContext(ctx).Output(data)
}

// PrintContext writes raw output to stdout using the logger from context.
// Use this for streaming output that already contains newlines.
func PrintContext(ctx context.Context, data string) {
	FromContext(ctx).Print(data)
}
