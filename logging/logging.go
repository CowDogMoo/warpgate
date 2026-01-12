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
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/fatih/color"
)

// logger is the global variable to access the custom logger.
var (
	logger     *CustomLogger
	loggerOnce sync.Once
	loggerMu   sync.RWMutex
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

func (l *CustomLogger) log(level LogLevel, message string, args ...interface{}) {
	formattedMsg := l.formatMessage(level, message, args...)

	if l.shouldShowOnConsole(level) && l.ConsoleWriter != nil {
		_, _ = fmt.Fprintln(l.ConsoleWriter, formattedMsg)
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

// SetQuiet enables or disables quiet mode.
// In quiet mode, only error messages are displayed.
func (l *CustomLogger) SetQuiet(quiet bool) {
	l.Quiet = quiet
}

// SetVerbose enables or disables verbose mode.
// In verbose mode, info and debug messages are displayed on console.
func (l *CustomLogger) SetVerbose(verbose bool) {
	l.Verbose = verbose
}

// Info logs an informational message.
func (l *CustomLogger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Output sends data to stdout.
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

// Initialize sets up the global logger following 12-factor app principles.
// Logs are written to stderr; the execution environment handles capture and routing.
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

	loggerMu.Lock()
	logger = &CustomLogger{
		LogLevel:      logLevel,
		OutputType:    outputType,
		Quiet:         quiet,
		ConsoleWriter: os.Stderr,
		Verbose:       verbose,
	}
	loggerMu.Unlock()

	return nil
}

// ensureLogger initializes the logger if it hasn't been initialized yet
func ensureLogger() error {
	loggerOnce.Do(func() {
		logger = &CustomLogger{
			LogLevel:      slog.LevelInfo,
			OutputType:    PlainOutput,
			Quiet:         false,
			ConsoleWriter: os.Stderr,
			Verbose:       false,
		}
	})
	return nil
}

// logGlobal handles logging through the global logger instance
func logGlobal(level LogLevel, message string, args ...interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	logger.log(level, message, args...)
}

// Info logs an informational message using the global logger.
func Info(message string, args ...interface{}) {
	logGlobal(InfoLevel, message, args...)
}

// Output sends data to stdout using the global logger.
func Output(data interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	logger.Output(data)
}

// Warn logs a warning message using the global logger.
func Warn(message string, args ...interface{}) {
	logGlobal(WarnLevel, message, args...)
}

// Error logs an error message using the global logger. It accepts either an error,
// a format string, or any other value as the first argument.
func Error(firstArg interface{}, args ...interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	logger.Error(firstArg, args...)
}

// Errorf logs a formatted error message using the global logger.
func Errorf(format string, args ...interface{}) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	logger.Errorf(format, args...)
}

// ErrorErr logs an error value using the global logger.
func ErrorErr(err error) {
	if initErr := ensureLogger(); initErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", initErr)
		return
	}

	loggerMu.RLock()
	defer loggerMu.RUnlock()
	logger.ErrorErr(err)
}

// Debug logs a debug message using the global logger.
func Debug(message string, args ...interface{}) {
	logGlobal(DebugLevel, message, args...)
}

// SetQuiet enables or disables quiet mode on the global logger.
// In quiet mode, only error messages are displayed.
func SetQuiet(quiet bool) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	loggerMu.Lock()
	defer loggerMu.Unlock()
	logger.SetQuiet(quiet)
}

// IsQuiet returns whether the global logger is in quiet mode.
func IsQuiet() bool {
	if err := ensureLogger(); err != nil {
		return false
	}
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return logger.Quiet
}

// SetVerbose enables or disables verbose mode on the global logger.
// In verbose mode, info and debug messages are displayed on console.
func SetVerbose(verbose bool) {
	if err := ensureLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return
	}
	loggerMu.Lock()
	defer loggerMu.Unlock()
	logger.SetVerbose(verbose)
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
// If no logger is found in context, returns the global logger.
func FromContext(ctx context.Context) *CustomLogger {
	if ctx == nil {
		if err := ensureLogger(); err != nil {
			return NewCustomLogger(slog.LevelInfo)
		}
		loggerMu.RLock()
		defer loggerMu.RUnlock()
		return logger
	}

	if l, ok := ctx.Value(loggerKey).(*CustomLogger); ok && l != nil {
		return l
	}

	// Fallback to global logger
	if err := ensureLogger(); err != nil {
		return NewCustomLogger(slog.LevelInfo)
	}
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return logger
}

// Context-aware logging functions
// These functions retrieve the logger from context and use it,
// falling back to the global logger if no context logger is available.

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
