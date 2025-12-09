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

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/fatih/color"
)

// logger is the global variable to access the custom logger.
var logger *CustomLogger

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

// Log levels for different types of log messages
const (
	// InfoLevel represents informational messages
	InfoLevel LogLevel = iota
	// WarnLevel represents warning messages
	WarnLevel
	// DebugLevel represents debug messages
	DebugLevel
	// ErrorLevel represents error messages
	ErrorLevel
)

// CustomLogger wraps the logging functionality with custom formatting options.
//
// **Attributes:**
//
// LogLevel: The current logging level.
// OutputType: The output type for the logger.
// Quiet: When true, suppresses non-error messages.
// ConsoleWriter: Writer for console output (stderr).
// Verbose: When true, shows info/debug messages on console.
type CustomLogger struct {
	LogLevel      slog.Level
	OutputType    OutputType
	Quiet         bool
	ConsoleWriter io.Writer
	Verbose       bool
}

// formatMessage handles color formatting based on output type and log level
func (l *CustomLogger) formatMessage(level LogLevel, message string, args ...interface{}) string {
	formattedMsg := fmt.Sprintf(message, args...)

	if l.OutputType != ColorOutput {
		return formattedMsg
	}

	colorFunc := map[LogLevel]func(format string, a ...interface{}) string{
		InfoLevel:  color.GreenString,
		WarnLevel:  color.YellowString,
		DebugLevel: color.CyanString,
		ErrorLevel: color.RedString,
	}[level]

	if colorFunc == nil {
		return formattedMsg
	}

	return colorFunc("%s", formattedMsg)
}

// shouldShowOnConsole determines if a message should be shown on console
func (l *CustomLogger) shouldShowOnConsole(level LogLevel) bool {
	// Always suppress in quiet mode except errors
	if l.Quiet && level != ErrorLevel {
		return false
	}

	// Check against log level
	var slogLevel slog.Level
	switch level {
	case InfoLevel:
		slogLevel = slog.LevelInfo
	case WarnLevel:
		slogLevel = slog.LevelWarn
	case DebugLevel:
		slogLevel = slog.LevelDebug
	case ErrorLevel:
		slogLevel = slog.LevelError
	}

	// Always show errors and warnings
	if level == ErrorLevel || level == WarnLevel {
		return true
	}

	// Info messages: show by default unless log level is set higher than Info
	if level == InfoLevel {
		return l.LogLevel <= slogLevel
	}

	// Debug messages: only show if verbose is enabled AND log level allows it
	return (l.Verbose || l.LogLevel <= slog.LevelDebug) && l.LogLevel <= slogLevel
}

// log handles the common logging logic for all levels.
// All log messages are sent to stderr following 12-factor app principles.
// The execution environment is responsible for capturing and routing logs.
func (l *CustomLogger) log(level LogLevel, message string, args ...interface{}) {
	formattedMsg := l.formatMessage(level, message, args...)

	// Write to stderr if appropriate based on log level and verbosity settings
	if l.shouldShowOnConsole(level) && l.ConsoleWriter != nil {
		_, _ = fmt.Fprintln(l.ConsoleWriter, formattedMsg)
	}
}

// NewCustomLogger creates a new instance of CustomLogger.
//
// **Parameters:**
//
// level: The logging level to be set for the logger.
//
// **Returns:**
//
// *CustomLogger: A pointer to the newly created CustomLogger instance.
func NewCustomLogger(level slog.Level) *CustomLogger {
	return &CustomLogger{
		LogLevel:      level,
		Quiet:         false,
		ConsoleWriter: os.Stderr, // Default to stderr for CLI output
		Verbose:       false,
		OutputType:    PlainOutput,
	}
}

// SetQuiet enables or disables quiet mode.
// In quiet mode, only error messages are displayed.
//
// **Parameters:**
//
// quiet: Boolean flag to enable/disable quiet mode.
func (l *CustomLogger) SetQuiet(quiet bool) {
	l.Quiet = quiet
}

// SetVerbose enables or disables verbose mode.
// In verbose mode, info and debug messages are displayed on console.
//
// **Parameters:**
//
// verbose: Boolean flag to enable/disable verbose mode.
func (l *CustomLogger) SetVerbose(verbose bool) {
	l.Verbose = verbose
}

// Info logs an informational message.
//
// **Parameters:**
//
// format: The format string for the log message.
// args: The arguments to be formatted into the log message.
func (l *CustomLogger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Output sends data to stdout.
// Supports JSON output when OutputType is JSONOutput.
// Use this for actual program output that can be piped to other commands.
// This is ALWAYS shown regardless of quiet/verbose settings.
//
// **Parameters:**
//
// data: The data to be output. Can be any type that's JSON-serializable.
func (l *CustomLogger) Output(data interface{}) {
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

// Warn logs a warning message.
//
// **Parameters:**
//
// format: The format string for the log message.
// args: The arguments to be formatted into the log message.
func (l *CustomLogger) Warn(format string, args ...interface{}) {
	l.log(WarnLevel, format, args...)
}

// Debug logs a debug message.
//
// **Parameters:**
//
// format: The format string for the log message.
// args: The arguments to be formatted into the log message.
func (l *CustomLogger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Error logs an error message.
//
// **Parameters:**
//
// firstArg: The first argument, which can be a string, an error, or any other type.
// args: Additional arguments to be formatted into the log message.
func (l *CustomLogger) Error(firstArg interface{}, args ...interface{}) {
	var format string
	switch v := firstArg.(type) {
	case error:
		if len(args) == 0 {
			format = v.Error()
		} else {
			format = fmt.Sprintf(v.Error(), args...)
		}
	case string:
		format = v
	default:
		format = fmt.Sprintf("%v", v)
	}

	l.log(ErrorLevel, format, args...)
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

// Initialize sets up the global logger following 12-factor app principles.
// Logs are written to stderr; the execution environment handles capture and routing.
//
// **Parameters:**
//
// logLevelStr: The log level as a string from the configuration.
// outputFormat: The output format (text, json, color)
// quiet: Whether to enable quiet mode
// verbose: Whether to enable verbose mode
//
// **Returns:**
//
// error: An error if the logger initialization fails.
func Initialize(logLevelStr, outputFormat string, quiet, verbose bool) error {
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

	logger = &CustomLogger{
		LogLevel:      logLevel,
		OutputType:    outputType,
		Quiet:         quiet,
		ConsoleWriter: os.Stderr,
		Verbose:       verbose,
	}

	return nil
}

// ensureLogger initializes the logger if it hasn't been initialized yet
func ensureLogger() error {
	if logger != nil {
		return nil
	}

	logger = &CustomLogger{
		LogLevel:      slog.LevelInfo,
		OutputType:    PlainOutput,
		Quiet:         false,
		ConsoleWriter: os.Stderr,
		Verbose:       false,
	}

	return nil
}

// logGlobal handles logging through the global logger instance
func logGlobal(level LogLevel, message string, args ...interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	logger.log(level, message, args...)
}

// Info logs an informational message using the global logger.
//
// **Parameters:**
//
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func Info(message string, args ...interface{}) {
	logGlobal(InfoLevel, message, args...)
}

// Output sends data to stdout using the global logger.
// Supports JSON output when configured. Use for program output.
// This is ALWAYS shown regardless of quiet/verbose settings.
//
// **Parameters:**
//
// data: The data to be output. Can be any type that's JSON-serializable.
func Output(data interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	logger.Output(data)
}

// Warn logs a warning message using the global logger.
//
// **Parameters:**
//
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func Warn(message string, args ...interface{}) {
	logGlobal(WarnLevel, message, args...)
}

// Error logs an error message using the global logger.
//
// **Parameters:**
//
// firstArg: The first argument, which can be a string, an error, or any other type.
// args: Additional arguments to be formatted into the log message.
func Error(firstArg interface{}, args ...interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}

	logger.Error(firstArg, args...)
}

// Debug logs a debug message using the global logger.
//
// **Parameters:**
//
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func Debug(message string, args ...interface{}) {
	logGlobal(DebugLevel, message, args...)
}

// SetQuiet enables or disables quiet mode on the global logger.
// In quiet mode, only error messages are displayed.
//
// **Parameters:**
//
// quiet: Boolean flag to enable/disable quiet mode.
func SetQuiet(quiet bool) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	logger.SetQuiet(quiet)
}

// SetVerbose enables or disables verbose mode on the global logger.
// In verbose mode, info and debug messages are displayed on console.
//
// **Parameters:**
//
// verbose: Boolean flag to enable/disable verbose mode.
func SetVerbose(verbose bool) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	logger.SetVerbose(verbose)
}

// Context-based logging support
// This provides an alternative to global logging for better testability

// loggerKeyType is the type for the logger context key
type loggerKeyType struct{}

// loggerKey is the context key for storing the logger
var loggerKey = loggerKeyType{}

// WithLogger returns a new context with the provided logger.
// This allows different parts of the application to use different logger configurations.
//
// **Parameters:**
//
// ctx: The parent context.
// l: The CustomLogger instance to store in the context.
//
// **Returns:**
//
// context.Context: A new context with the logger attached.
func WithLogger(ctx context.Context, l *CustomLogger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves the logger from the context.
// If no logger is found in context, returns the global logger.
//
// **Parameters:**
//
// ctx: The context to retrieve the logger from.
//
// **Returns:**
//
// *CustomLogger: The logger from context, or the global logger as fallback.
func FromContext(ctx context.Context) *CustomLogger {
	if ctx == nil {
		if err := ensureLogger(); err != nil {
			return NewCustomLogger(slog.LevelInfo)
		}
		return logger
	}

	if l, ok := ctx.Value(loggerKey).(*CustomLogger); ok && l != nil {
		return l
	}

	// Fallback to global logger
	if err := ensureLogger(); err != nil {
		return NewCustomLogger(slog.LevelInfo)
	}
	return logger
}

// Context-aware logging functions
// These functions retrieve the logger from context and use it,
// falling back to the global logger if no context logger is available.

// InfoContext logs an informational message using the logger from context.
//
// **Parameters:**
//
// ctx: The context containing the logger.
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func InfoContext(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).Info(message, args...)
}

// WarnContext logs a warning message using the logger from context.
//
// **Parameters:**
//
// ctx: The context containing the logger.
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func WarnContext(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).Warn(message, args...)
}

// DebugContext logs a debug message using the logger from context.
//
// **Parameters:**
//
// ctx: The context containing the logger.
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func DebugContext(ctx context.Context, message string, args ...interface{}) {
	FromContext(ctx).Debug(message, args...)
}

// ErrorContext logs an error message using the logger from context.
//
// **Parameters:**
//
// ctx: The context containing the logger.
// firstArg: The first argument, which can be a string, an error, or any other type.
// args: Additional arguments to be formatted into the log message.
func ErrorContext(ctx context.Context, firstArg interface{}, args ...interface{}) {
	FromContext(ctx).Error(firstArg, args...)
}

// OutputContext sends data to stdout using the logger from context.
//
// **Parameters:**
//
// ctx: The context containing the logger.
// data: The data to be output.
func OutputContext(ctx context.Context, data interface{}) {
	FromContext(ctx).Output(data)
}
