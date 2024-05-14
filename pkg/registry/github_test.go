package registry_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/registry"
)

type MockHttpClient struct {
	Response *http.Response
	Err      error
}

func (c *MockHttpClient) Do(req *http.Request) (*http.Response, error) {
	return c.Response, c.Err
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "valid token",
			token:        "valid_token",
			statusCode:   http.StatusOK,
			responseBody: `OK`,
			expectError:  false,
		},
		{
			name:         "invalid token",
			token:        "invalid_token",
			statusCode:   http.StatusUnauthorized,
			responseBody: `Unauthorized`,
			expectError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry.Client = &MockHttpClient{
				Response: &http.Response{
					StatusCode: tc.statusCode,
					Body:       io.NopCloser(bytes.NewBufferString(tc.responseBody)),
				},
				Err: nil,
			}
			defer func() { registry.Client = &http.Client{} }()

			err := registry.ValidateToken(tc.token)
			if (err != nil) != tc.expectError {
				t.Errorf("validateToken() for %v, error expected: %v, got: %v",
					tc.token, tc.expectError, err)
			}
		})
	}
}
