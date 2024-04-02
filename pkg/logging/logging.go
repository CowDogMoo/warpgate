package logging

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/fatih/color"
	log "github.com/l50/goutils/v2/logging"
	"github.com/spf13/afero"
)

// logger is the global variable to access the custom logger.
var logger *CustomLogger

// CustomLogger is a wrapper around the goutils logger.
//
// **Attributes:**
//
// Logger: The underlying logger instance.
// LogLevel: The current logging level.
type CustomLogger struct {
	Logger   log.Logger
	LogLevel slog.Level
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
		Logger:   log.L(),
		LogLevel: level,
	}
}

// Info logs an informational message.
//
// **Parameters:**
//
// format: The format string for the log message.
// args: The arguments to be formatted into the log message.
func (l *CustomLogger) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.Logger.Printf(color.CyanString(message))
}

// Warn logs a warning message.
//
// **Parameters:**
//
// format: The format string for the log message.
// args: The arguments to be formatted into the log message.
func (l *CustomLogger) Warn(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.Logger.Warnf(color.YellowString(message))
}

// Debug logs a debug message.
//
// **Parameters:**
//
// format: The format string for the log message.
// args: The arguments to be formatted into the log message.
func (l *CustomLogger) Debug(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.Logger.Debugf(color.BlueString(message))
}

// Error logs an error message.
//
// **Parameters:**
//
// firstArg: The first argument, which can be a string, an error, or any other type.
// args: Additional arguments to be formatted into the log message.
func (l *CustomLogger) Error(firstArg interface{}, args ...interface{}) {
	var message string
	switch v := firstArg.(type) {
	case error:
		if len(args) == 0 {
			message = v.Error()
		} else {
			message = fmt.Sprintf(v.Error(), args...)
		}
	case string:
		message = fmt.Sprintf(v, args...)
	default:
		message = fmt.Sprintf("%v", v)
	}
	l.Logger.Errorf(color.RedString(message))
}

// Initialize sets up the global logger.
//
// **Parameters:**
//
// configDir: The directory where the log file will be stored.
// logLevelStr: The log level as a string from the configuration.
//
// **Returns:**
//
// error: An error if the logger initialization fails.
func Initialize(configDir, logLevelStr, logName string) error {
	// Initialize the global logger
	logger = NewCustomLogger(log.DetermineLogLevel(logLevelStr))

	logLevel := log.DetermineLogLevel(logLevelStr)

	logCfg := log.LogConfig{
		Fs:         afero.NewOsFs(),
		LogPath:    filepath.Join(configDir, logName),
		Level:      logLevel,
		OutputType: log.ColorOutput,
		LogToDisk:  true,
	}

	var err error
	logger.Logger, err = log.InitLogging(&logCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize the logger: %v", err)
	}

	// Set the global logger
	log.GlobalLogger = logger.Logger

	// Update the log level in the custom logger
	logger.LogLevel = logCfg.Level

	return nil
}

// Info logs an informational message using the global logger.
//
// **Parameters:**
//
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func Info(message string, args ...interface{}) {
	logger.Info(message, args...)
}

// Warn logs a warning message using the global logger.
//
// **Parameters:**
//
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func Warn(message string, args ...interface{}) {
	logger.Warn(message, args...)
}

// Error logs an error message using the global logger.
//
// **Parameters:**
//
// firstArg: The first argument, which can be a string, an error, or any other type.
// args: Additional arguments to be formatted into the log message.
func Error(firstArg interface{}, args ...interface{}) {
	logger.Error(firstArg, args...)
}

// Debug logs a debug message using the global logger.
//
// **Parameters:**
//
// message: The format string for the log message.
// args: The arguments to be formatted into the log message.
func Debug(message string, args ...interface{}) {
	logger.Debug(message, args...)
}
