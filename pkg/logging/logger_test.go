package logging_test

import (
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
