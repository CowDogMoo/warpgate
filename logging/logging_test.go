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

package logging_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/logging"
)

func TestNewCustomLogger(t *testing.T) {
	tests := []struct {
		name      string
		level     slog.Level
		wantLevel slog.Level
		wantQuiet bool
	}{
		{
			name:      "info level",
			level:     slog.LevelInfo,
			wantLevel: slog.LevelInfo,
			wantQuiet: false,
		},
		{
			name:      "debug level",
			level:     slog.LevelDebug,
			wantLevel: slog.LevelDebug,
			wantQuiet: false,
		},
		{
			name:      "error level",
			level:     slog.LevelError,
			wantLevel: slog.LevelError,
			wantQuiet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.NewCustomLogger(tt.level)
			if logger == nil {
				t.Fatal("expected non-nil logger")
				return
			}
			if logger.LogLevel != tt.wantLevel {
				t.Errorf("got level %v, want %v",
					logger.LogLevel, tt.wantLevel)
			}
			if logger.Quiet != tt.wantQuiet {
				t.Errorf("got quiet %v, want %v",
					logger.Quiet, tt.wantQuiet)
			}
		})
	}
}

func TestNewCustomLoggerWithOptions(t *testing.T) {
	tests := []struct {
		name         string
		logLevel     string
		outputFormat string
		quiet        bool
		verbose      bool
		wantLevel    slog.Level
		wantOutput   logging.OutputType
		wantQuiet    bool
		wantVerbose  bool
	}{
		{
			name:         "default settings",
			logLevel:     "info",
			outputFormat: "text",
			quiet:        false,
			verbose:      false,
			wantLevel:    slog.LevelInfo,
			wantOutput:   logging.PlainOutput,
			wantQuiet:    false,
			wantVerbose:  false,
		},
		{
			name:         "json format",
			logLevel:     "debug",
			outputFormat: "json",
			quiet:        true,
			verbose:      false,
			wantLevel:    slog.LevelDebug,
			wantOutput:   logging.JSONOutput,
			wantQuiet:    true,
			wantVerbose:  false,
		},
		{
			name:         "color format with verbose",
			logLevel:     "warn",
			outputFormat: "color",
			quiet:        false,
			verbose:      true,
			wantLevel:    slog.LevelDebug, // verbose forces debug level
			wantOutput:   logging.ColorOutput,
			wantQuiet:    false,
			wantVerbose:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.NewCustomLoggerWithOptions(tt.logLevel, tt.outputFormat, tt.quiet, tt.verbose)
			if logger == nil {
				t.Fatal("expected non-nil logger")
			}
			if logger.LogLevel != tt.wantLevel {
				t.Errorf("got level %v, want %v", logger.LogLevel, tt.wantLevel)
			}
			if logger.OutputType != tt.wantOutput {
				t.Errorf("got output type %v, want %v", logger.OutputType, tt.wantOutput)
			}
			if logger.Quiet != tt.wantQuiet {
				t.Errorf("got quiet %v, want %v", logger.Quiet, tt.wantQuiet)
			}
			if logger.Verbose != tt.wantVerbose {
				t.Errorf("got verbose %v, want %v", logger.Verbose, tt.wantVerbose)
			}
		})
	}
}

func TestCustomLogger_SetQuiet(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)

	logger.SetQuiet(true)
	if !logger.Quiet {
		t.Error("expected quiet mode to be enabled")
	}

	logger.SetQuiet(false)
	if logger.Quiet {
		t.Error("expected quiet mode to be disabled")
	}
}

func TestCustomLogger_SetVerbose(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)

	logger.SetVerbose(true)
	if !logger.Verbose {
		t.Error("expected verbose mode to be enabled")
	}

	logger.SetVerbose(false)
	if logger.Verbose {
		t.Error("expected verbose mode to be disabled")
	}
}

func TestCustomLogger_IsQuiet(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)

	if logger.IsQuiet() {
		t.Error("expected IsQuiet to return false by default")
	}

	logger.SetQuiet(true)
	if !logger.IsQuiet() {
		t.Error("expected IsQuiet to return true after SetQuiet(true)")
	}
}

func TestCustomLogger_Error(t *testing.T) {
	tests := []struct {
		name     string
		firstArg interface{}
		args     []interface{}
		wantMsg  string
	}{
		{
			name:     "error type",
			firstArg: errors.New("test error"),
			args:     []interface{}{},
			wantMsg:  "test error",
		},
		{
			name:     "string format",
			firstArg: "error: %s",
			args:     []interface{}{"failed"},
			wantMsg:  "error: failed",
		},
		{
			name:     "other type",
			firstArg: 42,
			args:     []interface{}{},
			wantMsg:  "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logging.NewCustomLogger(slog.LevelError)
			// Just ensure it doesn't panic
			logger.Error(tt.firstArg, tt.args...)
		})
	}
}

func TestCustomLogger_Info(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)

	// Normal mode - should log
	logger.SetQuiet(false)
	logger.Info("test message %d", 42)

	// Quiet mode - should not log
	logger.SetQuiet(true)
	logger.Info("should not appear")
}

func TestCustomLogger_Warn(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)
	logger.Warn("warning message %s", "test")
}

func TestCustomLogger_Debug(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelDebug)
	logger.Debug("debug message")
}

func TestCustomLogger_Errorf(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelError)
	// Ensure it doesn't panic with various format strings
	logger.Errorf("simple error message")
	logger.Errorf("error with arg: %s", "test")
	logger.Errorf("error with multiple args: %s %d", "test", 42)
}

func TestCustomLogger_ErrorErr(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelError)
	// Ensure it doesn't panic with error values
	logger.ErrorErr(errors.New("test error"))
	logger.ErrorErr(nil) // Should handle nil gracefully
}

func TestDetermineLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		levelStr  string
		wantLevel slog.Level
	}{
		{
			name:      "debug level",
			levelStr:  "debug",
			wantLevel: slog.LevelDebug,
		},
		{
			name:      "info level",
			levelStr:  "info",
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "warn level",
			levelStr:  "warn",
			wantLevel: slog.LevelWarn,
		},
		{
			name:      "error level",
			levelStr:  "error",
			wantLevel: slog.LevelError,
		},
		{
			name:      "unknown level defaults to info",
			levelStr:  "unknown",
			wantLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logging.DetermineLogLevel(tt.levelStr)
			if got != tt.wantLevel {
				t.Errorf("got level %v, want %v", got, tt.wantLevel)
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelDebug)
	logger.SetQuiet(true)

	ctx := context.Background()
	ctx = logging.WithLogger(ctx, logger)

	// Retrieve logger from context
	retrieved := logging.FromContext(ctx)
	if retrieved == nil {
		t.Fatal("expected non-nil logger from context")
	}
	if !retrieved.IsQuiet() {
		t.Error("expected retrieved logger to have quiet mode enabled")
	}
	if retrieved.LogLevel != slog.LevelDebug {
		t.Errorf("got level %v, want %v", retrieved.LogLevel, slog.LevelDebug)
	}
}

func TestFromContext_NilContext(t *testing.T) {
	// Should return a default logger when context is nil
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logger := logging.FromContext(nil)
	if logger == nil {
		t.Fatal("expected non-nil logger even with nil context")
	}
	// Default logger should have info level
	if logger.LogLevel != slog.LevelInfo {
		t.Errorf("got level %v, want %v", logger.LogLevel, slog.LevelInfo)
	}
}

func TestFromContext_NoLogger(t *testing.T) {
	// Should return a default logger when context has no logger
	ctx := context.Background()
	logger := logging.FromContext(ctx)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.LogLevel != slog.LevelInfo {
		t.Errorf("got level %v, want %v", logger.LogLevel, slog.LevelInfo)
	}
}

func TestContextLogging(t *testing.T) {
	// Create a logger that writes to a buffer so we can verify output
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLogger(slog.LevelDebug)
	logger.ConsoleWriter = buf
	logger.SetVerbose(true)

	ctx := logging.WithLogger(context.Background(), logger)

	// Test all context logging functions don't panic
	logging.InfoContext(ctx, "info message %d", 42)
	logging.WarnContext(ctx, "warn message %s", "test")
	logging.DebugContext(ctx, "debug message")
	logging.ErrorContext(ctx, "error message")
	logging.ErrorfContext(ctx, "formatted error: %s", "test")
	logging.ErrorErrContext(ctx, errors.New("test error"))

	// Verify something was written
	if buf.Len() == 0 {
		t.Error("expected output to be written to buffer")
	}
}

func TestContextLogging_NilContext(t *testing.T) {
	// Should not panic with nil context - uses default logger
	// All calls below deliberately pass nil to test graceful handling
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logging.InfoContext(nil, "info message")
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logging.WarnContext(nil, "warn message")
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logging.DebugContext(nil, "debug message")
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logging.ErrorContext(nil, "error message")
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logging.ErrorfContext(nil, "formatted error")
	//nolint:staticcheck // SA1012: deliberately testing nil context handling
	logging.ErrorErrContext(nil, errors.New("test error"))
}

func TestOutputContext(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLogger(slog.LevelInfo)
	logger.ConsoleWriter = buf

	ctx := logging.WithLogger(context.Background(), logger)

	// Just ensure it doesn't panic - Output goes to stdout, not the buffer
	logging.OutputContext(ctx, "test output")
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level logging.LogLevel
		want  string
	}{
		{logging.DebugLevel, "DEBUG"},
		{logging.InfoLevel, "INFO"},
		{logging.WarnLevel, "WARN"},
		{logging.ErrorLevel, "ERROR"},
		{logging.LogLevel(99), "INFO"}, // unknown level defaults to INFO
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.want {
				t.Errorf("LogLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestLogLevel_Ordering(t *testing.T) {
	// Verify log levels are ordered from least to most severe
	if logging.DebugLevel >= logging.InfoLevel {
		t.Error("DebugLevel should be less than InfoLevel")
	}
	if logging.InfoLevel >= logging.WarnLevel {
		t.Error("InfoLevel should be less than WarnLevel")
	}
	if logging.WarnLevel >= logging.ErrorLevel {
		t.Error("WarnLevel should be less than ErrorLevel")
	}
}

func TestCustomLogger_ConcurrentAccess(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)
	buf := &bytes.Buffer{}
	logger.ConsoleWriter = buf

	// Run concurrent operations to verify thread safety
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				logger.SetQuiet(true)
				logger.SetQuiet(false)
				logger.SetVerbose(true)
				logger.SetVerbose(false)
				_ = logger.IsQuiet()
				logger.Info("concurrent message %d", j)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPrintContext(t *testing.T) {
	logger := logging.NewCustomLogger(slog.LevelInfo)

	ctx := logging.WithLogger(context.Background(), logger)

	// Just ensure it doesn't panic - Print goes to stdout
	logging.PrintContext(ctx, "test print output\n")
}

func TestCustomLogger_ColorOutput_AllLevels(t *testing.T) {
	// Test that ColorOutput mode writes colored output for all log levels.
	// This exercises the formatMessage ColorOutput branches.
	tests := []struct {
		name    string
		logFunc func(*logging.CustomLogger)
		prefix  string
	}{
		{
			name:    "debug level color output",
			logFunc: func(l *logging.CustomLogger) { l.Debug("debug msg") },
			prefix:  "DEBUG",
		},
		{
			name:    "info level color output",
			logFunc: func(l *logging.CustomLogger) { l.Info("info msg") },
			prefix:  "INFO",
		},
		{
			name:    "warn level color output",
			logFunc: func(l *logging.CustomLogger) { l.Warn("warn msg") },
			prefix:  "WARN",
		},
		{
			name:    "error level color output",
			logFunc: func(l *logging.CustomLogger) { l.Error("error msg") },
			prefix:  "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := logging.NewCustomLoggerWithOptions("debug", "color", false, true)
			logger.ConsoleWriter = buf

			tt.logFunc(logger)

			output := buf.String()
			if output == "" {
				t.Error("expected output to be written to buffer")
			}
			// The color output should contain the level prefix somewhere
			// (color codes wrap it but the text is still present)
			if !bytes.Contains(buf.Bytes(), []byte(tt.prefix)) {
				t.Errorf("expected output to contain %q, got %q", tt.prefix, output)
			}
		})
	}
}

func TestCustomLogger_PlainOutput_NoColorPrefix(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLoggerWithOptions("debug", "plain", false, true)
	logger.ConsoleWriter = buf

	logger.Info("plain message")

	output := buf.String()
	if output == "" {
		t.Error("expected output to be written")
	}
	// Plain output should contain the message without [INFO] prefix
	if !bytes.Contains(buf.Bytes(), []byte("plain message")) {
		t.Errorf("expected output to contain 'plain message', got %q", output)
	}
}

func TestCustomLogger_QuietMode_OnlyErrors(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLogger(slog.LevelDebug)
	logger.ConsoleWriter = buf
	logger.SetQuiet(true)

	logger.Debug("debug should not appear")
	logger.Info("info should not appear")
	logger.Warn("warn should not appear")
	output := buf.String()
	if output != "" {
		t.Errorf("expected no output in quiet mode for non-error, got %q", output)
	}

	logger.Error("error should appear")
	output = buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("error should appear")) {
		t.Errorf("expected error to appear in quiet mode, got %q", output)
	}
}

func TestCustomLogger_VerboseMode_ShowsDebug(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLogger(slog.LevelDebug)
	logger.ConsoleWriter = buf
	logger.SetVerbose(true)

	logger.Debug("verbose debug message")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("verbose debug message")) {
		t.Errorf("expected debug message in verbose mode, got %q", output)
	}
}

func TestCustomLogger_DefaultMode_HidesDebug(t *testing.T) {
	// Default mode (not verbose, not quiet) should hide debug messages
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLogger(slog.LevelDebug)
	logger.ConsoleWriter = buf
	// Default: verbose=false, quiet=false

	logger.Debug("debug should be hidden")
	output := buf.String()
	if output != "" {
		t.Errorf("expected debug to be hidden in default mode, got %q", output)
	}

	logger.Info("info should appear")
	output = buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("info should appear")) {
		t.Errorf("expected info to appear in default mode, got %q", output)
	}
}

func TestCustomLogger_NilConsoleWriter(t *testing.T) {
	// When ConsoleWriter is nil, log should not panic
	logger := logging.NewCustomLogger(slog.LevelInfo)
	logger.ConsoleWriter = nil

	// These should not panic
	logger.Info("test with nil writer")
	logger.Error("error with nil writer")
}

func TestNewCustomLoggerWithOptions_PlainFormat(t *testing.T) {
	logger := logging.NewCustomLoggerWithOptions("info", "plain", false, false)
	if logger.OutputType != logging.PlainOutput {
		t.Errorf("expected PlainOutput for 'plain' format, got %v", logger.OutputType)
	}
}

func TestNewCustomLoggerWithOptions_VerboseOverridesLevel(t *testing.T) {
	// When verbose is set and level is already debug, it should stay debug
	logger := logging.NewCustomLoggerWithOptions("debug", "text", false, true)
	if logger.LogLevel != slog.LevelDebug {
		t.Errorf("expected LevelDebug, got %v", logger.LogLevel)
	}
	if !logger.Verbose {
		t.Error("expected verbose to be true")
	}
}

func TestNewCustomLoggerWithOptions_UnknownFormat(t *testing.T) {
	// Unknown format should default to PlainOutput
	logger := logging.NewCustomLoggerWithOptions("info", "unknown-format", false, false)
	if logger.OutputType != logging.PlainOutput {
		t.Errorf("expected PlainOutput for unknown format, got %v", logger.OutputType)
	}
}

func TestCustomLogger_ErrorWithFormatArgs(t *testing.T) {
	// Test Error with an error value that has format args
	buf := &bytes.Buffer{}
	logger := logging.NewCustomLogger(slog.LevelError)
	logger.ConsoleWriter = buf

	err := errors.New("base error: %s placeholder")
	logger.Error(err, "extra arg")

	output := buf.String()
	if output == "" {
		t.Error("expected output from Error with error and args")
	}
}
