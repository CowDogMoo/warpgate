package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/fatih/color"
	slogmulti "github.com/samber/slog-multi"
	"github.com/spf13/afero"
)

// GlobalLogger is a variable that holds the instance of the logger.
var GlobalLogger Logger

// InitGlobalLogger initializes the global logger with the specified level and file path.
// This function should be called at the beginning of your application.
func InitGlobalLogger(level slog.Level, path string) error {
	var err error
	GlobalLogger, err = configureLogger(level, path)
	return err
}

// L returns the global logger instance.
func L() Logger {
	return GlobalLogger
}

// Logger is an interface that defines methods for a generic logging
// system. It supports basic logging operations like printing,
// formatted printing, error logging, and debug logging.
//
// **Methods:**
//
// Println: Outputs a line with the given arguments.
// Printf: Outputs a formatted string.
// Error: Logs an error message.
// Errorf: Logs a formatted error message.
// Debug: Logs a debug message.
// Debugf: Logs a formatted debug message.
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
}

// ColoredLogger is a logger that outputs messages in a specified color.
// It enhances readability by color-coding log messages based on their
// severity or purpose.
//
// **Attributes:**
//
// Info: LogInfo object containing information about the log file.
// ColorAttribute: A color attribute for output styling.
type ColoredLogger struct {
	Info           LogInfo
	ColorAttribute color.Attribute
}

// Println logs a line with the provided arguments in the specified color.
func (l *ColoredLogger) Println(v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Println(color.New(l.ColorAttribute).Sprint(v...))
}

// Printf logs a formatted string in the specified color.
func (l *ColoredLogger) Printf(format string, v ...interface{}) {
	log.SetOutput(l.Info.File)
	if len(v) > 0 {
		log.Println(color.New(l.ColorAttribute).Sprintf(format, v...))
	} else {
		log.Println(color.New(l.ColorAttribute).Sprint(format))
	}
}

// Error logs an error message in bold and the specified color.
func (l *ColoredLogger) Error(v ...interface{}) {
	log.SetOutput(l.Info.File)
	combined := append([]interface{}{"ERROR:"}, v...)
	log.Println(color.New(l.ColorAttribute).Add(color.Bold).Sprint(combined...))
}

// Errorf logs a formatted error message in bold and the specified color.
func (l *ColoredLogger) Errorf(format string, v ...interface{}) {
	log.SetOutput(l.Info.File)
	formatted := fmt.Sprintf("ERROR: "+format, v...)
	log.Println(color.New(l.ColorAttribute).Add(color.Bold).Sprint(formatted))
}

// Debug logs a debug message in the specified color.
func (l *ColoredLogger) Debug(v ...interface{}) {
	combined := append([]interface{}{"DEBUG:"}, v...)
	fmt.Println(color.New(l.ColorAttribute).Sprint(combined...))
}

// Debugf logs a formatted debug message in the specified color.
func (l *ColoredLogger) Debugf(format string, v ...interface{}) {
	formatted := fmt.Sprintf("DEBUG: "+format, v...)
	fmt.Println(color.New(l.ColorAttribute).Sprint(formatted))
}

// PlainLogger is a logger that outputs messages in plain text format.
// It is a basic logger without additional styling or color-coding.
//
// **Attributes:**
//
// Info: LogInfo object containing information about the log file.
type PlainLogger struct {
	Info LogInfo
}

// Println logs a line with the provided arguments in plain text format.
func (l *PlainLogger) Println(v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Println(v...)
}

// Printf logs a formatted string in plain text format.
func (l *PlainLogger) Printf(format string, v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Printf(format, v...)
}

// Error logs an error message in plain text format.
func (l *PlainLogger) Error(v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Println("ERROR:", fmt.Sprint(v...))
}

// Errorf logs a formatted error message in plain text format.
func (l *PlainLogger) Errorf(format string, v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Printf("ERROR: "+format, v...)
}

// Debug logs a debug message in plain text format.
func (l *PlainLogger) Debug(v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Println("DEBUG:", fmt.Sprint(v...))
}

// Debugf logs a formatted debug message in plain text format.
func (l *PlainLogger) Debugf(format string, v ...interface{}) {
	log.SetOutput(l.Info.File)
	log.Printf("DEBUG: "+format, v...)
}

// LogInfo holds parameters used to manage logging throughout a program.
// It includes details such as log directory, file, and file name.
//
// **Attributes:**
//
// Dir: Directory where the log file is located.
// File: File object representing the log file.
// FileName: Name of the log file.
// Path: Full path to the log file.
type LogInfo struct {
	Dir      string
	File     afero.File
	FileName string
	Path     string
}

// CreateLogFile creates a log file in a specified directory. It ensures
// the directory exists and creates a new log file if it doesn't exist.
//
// **Parameters:**
//
// fs: Filesystem interface for file operations.
// logDir: Directory to create the log file in.
// logName: Name of the log file to create.
//
// **Returns:**
//
// LogInfo: Information about the created log file.
// error: An error if there is a failure in creating the log file.
func CreateLogFile(fs afero.Fs, logDir string, logName string) (LogInfo, error) {
	logInfo := LogInfo{}
	var err error

	logDir = strings.TrimSpace(logDir)
	logName = strings.TrimSpace(logName)

	if logDir == "" {
		return logInfo, fmt.Errorf("logDir cannot be empty")
	}

	if logName == "" {
		return logInfo, fmt.Errorf("logName cannot be empty")
	}

	logInfo.Dir = filepath.Join(logDir, "logs")

	if filepath.Ext(logName) != ".log" {
		logInfo.FileName = fmt.Sprintf("%s.log", logName)
	} else {
		logInfo.FileName = logName
	}

	logInfo.Path = filepath.Join(logInfo.Dir, logInfo.FileName)

	if _, err := fs.Stat(logInfo.Path); os.IsNotExist(err) {
		if err := fs.MkdirAll(logInfo.Dir, os.ModePerm); err != nil {
			return logInfo, fmt.Errorf("failed to create %s: %v", logInfo.Dir, err)
		}
	}

	logInfo.File, err = fs.OpenFile(logInfo.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return logInfo, fmt.Errorf("failed to create %s: %v", logInfo.Path, err)
	}

	return logInfo, nil
}

// SlogLogger is a logger implementation using the slog library. It
// provides structured logging capabilities.
//
// **Attributes:**
//
// Logger: The slog Logger instance used for logging operations.
type SlogLogger struct {
	Logger *slog.Logger
}

// Println logs a line with the provided arguments using slog library.
func (l *SlogLogger) Println(v ...interface{}) {
	l.Logger.Info(fmt.Sprint(v...))
}

// Printf logs a formatted string using slog library.
func (l *SlogLogger) Printf(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

// Error logs an error message using slog library.
func (l *SlogLogger) Error(v ...interface{}) {
	l.Logger.Error(fmt.Sprint(v...))
}

// Errorf logs a formatted error message using slog library.
func (l *SlogLogger) Errorf(format string, v ...interface{}) {
	l.Logger.Error(fmt.Sprintf(format, v...))
}

// Debug logs a debug message using slog library.
func (l *SlogLogger) Debug(v ...interface{}) {
	l.Logger.Debug(fmt.Sprint(v...))
}

// Debugf logs a formatted debug message using slog library.
func (l *SlogLogger) Debugf(format string, v ...interface{}) {
	l.Logger.Debug(fmt.Sprintf(format, v...))
}

// SlogPlainLogger is a simple logger using the slog library. It provides
// basic logging without structured formatting.
//
// **Attributes:**
//
// Logger: The slog Logger instance used for logging operations.
type SlogPlainLogger struct {
	Logger *slog.Logger
}

// Println logs a line with the provided arguments using slog library.
func (l *SlogPlainLogger) Println(v ...interface{}) {
	l.Logger.Info(fmt.Sprint(v...))
}

// Printf logs a formatted string using slog library.
func (l *SlogPlainLogger) Printf(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

// Error logs an error message using slog library.
func (l *SlogPlainLogger) Error(v ...interface{}) {
	l.Logger.Error(fmt.Sprint(v...))
}

// Errorf logs a formatted error message using slog library.
func (l *SlogPlainLogger) Errorf(format string, v ...interface{}) {
	l.Logger.Error(fmt.Sprintf(format, v...))
}

// Debug logs a debug message using slog library.
func (l *SlogPlainLogger) Debug(v ...interface{}) {
	l.Logger.Debug(fmt.Sprint(v...))
}

// Debugf logs a formatted debug message using slog library.
func (l *SlogPlainLogger) Debugf(format string, v ...interface{}) {
	l.Logger.Debug(fmt.Sprintf(format, v...))
}

// configureLogger creates and configures a logger based on the specified
// logging level and file path. It sets up the logger to write to both
// the file and standard output.
//
// **Parameters:**
//
// level: The logging level to set for the logger.
// path: Path to the log file.
//
// **Returns:**
//
// Logger: The configured logger instance.
// error: An error if the logger configuration fails.
func configureLogger(level slog.Level, path string) (Logger, error) {
	var err error

	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	opts := slog.HandlerOptions{Level: level}

	fileHandler := slog.NewJSONHandler(logFile, &opts)
	stdoutHandler := slog.NewJSONHandler(os.Stdout, &opts)
	handler := slogmulti.Fanout(fileHandler, stdoutHandler)

	logger := slog.New(handler)

	if level == slog.LevelDebug {
		return &SlogLogger{Logger: logger}, nil
	}
	return &SlogPlainLogger{Logger: logger}, nil
}
