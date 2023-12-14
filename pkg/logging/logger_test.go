package logging_test

import (
	"log/slog"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/logging"
	"github.com/spf13/afero"
)

func TestCreateLogFile(t *testing.T) {
	testCases := []struct {
		name      string
		logDir    string
		logName   string
		wantError bool
	}{
		{
			name:      "ValidLogCreation",
			logDir:    "/tmp/logs",
			logName:   "test.log",
			wantError: false,
		},
		{
			name:      "EmptyLogDir",
			logDir:    "",
			logName:   "test.log",
			wantError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			_, err := logging.CreateLogFile(fs, tc.logDir, tc.logName)
			if (err != nil) != tc.wantError {
				t.Errorf("CreateLogFile() error = %v, wantError %v", err, tc.wantError)
			}
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{
			name:     "DebugLevel",
			input:    "debug",
			expected: slog.LevelDebug,
		},
		{
			name:     "InfoLevel",
			input:    "info",
			expected: slog.LevelInfo,
		},
		{
			name:     "DefaultLevel",
			input:    "other",
			expected: slog.LevelInfo,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := logging.GetLogLevel(tc.input)
			if result != tc.expected {
				t.Errorf("GetLogLevel(%v) = %v, want %v",
					tc.input, result, tc.expected)
			}
		})
	}
}
