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

func TestInitialize(t *testing.T) {
	tests := []struct {
		name      string
		logLevel  string
		logFormat string
		quiet     bool
		verbose   bool
		wantErr   bool
	}{
		{
			name:      "valid initialization",
			logLevel:  "info",
			logFormat: "text",
			quiet:     false,
			verbose:   false,
			wantErr:   false,
		},
		{
			name:      "json format",
			logLevel:  "debug",
			logFormat: "json",
			quiet:     true,
			verbose:   false,
			wantErr:   false,
		},
		{
			name:      "color format",
			logLevel:  "warn",
			logFormat: "color",
			quiet:     false,
			verbose:   true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logging.Initialize(tt.logLevel, tt.logFormat, tt.quiet, tt.verbose)

			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v",
					err, tt.wantErr)
			}
		})
	}
}

func TestGlobalInfo(t *testing.T) {
	// Just ensure it doesn't panic
	logging.Info("test message %d", 42)
}

func TestGlobalWarn(t *testing.T) {
	// Just ensure it doesn't panic
	logging.Warn("warning message %s", "test")
}

func TestGlobalDebug(t *testing.T) {
	// Just ensure it doesn't panic
	logging.Debug("debug message")
}

func TestGlobalError(t *testing.T) {
	tests := []struct {
		name     string
		firstArg interface{}
		args     []interface{}
	}{
		{
			name:     "error type",
			firstArg: errors.New("test error"),
			args:     []interface{}{},
		},
		{
			name:     "string format",
			firstArg: "error: %s",
			args:     []interface{}{"failed"},
		},
		{
			name:     "integer type",
			firstArg: 42,
			args:     []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			logging.Error(tt.firstArg, tt.args...)
		})
	}
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
