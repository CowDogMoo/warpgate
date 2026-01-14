package logging_test

import (
	"testing"

	"github.com/cowdogmoo/warpgate/v3/logging"
)

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "URL without credentials",
			input: "https://github.com/org/repo.git",
			want:  "https://github.com/org/repo.git",
		},
		{
			name:  "URL with user and password",
			input: "https://user:password123@github.com/org/repo.git",
			want:  "https://***:***@github.com/org/repo.git",
		},
		{
			name:  "URL with token only",
			input: "https://ghp_tokenvalue@github.com/org/repo.git",
			want:  "https://***@github.com/org/repo.git",
		},
		{
			name:  "URL with x-access-token",
			input: "https://x-access-token:ghs_secrettoken@github.com/org/repo.git",
			want:  "https://***:***@github.com/org/repo.git",
		},
		{
			name:  "SSH URL unchanged",
			input: "git@github.com:org/repo.git",
			want:  "git@github.com:org/repo.git",
		},
		{
			name:  "HTTP URL with credentials",
			input: "http://admin:secret@localhost:8080/path",
			want:  "http://***:***@localhost:8080/path",
		},
		{
			name:  "URL with special chars in password",
			input: "https://user:p%40ss%3Dword@host.com/path",
			want:  "https://***:***@host.com/path",
		},
		{
			name:  "file URL unchanged",
			input: "file:///path/to/local/repo",
			want:  "file:///path/to/local/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logging.RedactURL(tt.input)
			if got != tt.want {
				t.Errorf("RedactURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		// Sensitive keys
		{"password", true},
		{"PASSWORD", true},
		{"db_password", true},
		{"secret", true},
		{"SECRET_KEY", true},
		{"my_secret", true},
		{"token", true},
		{"api_token", true},
		{"TOKEN_VALUE", true},
		{"credential", true},
		{"credentials", true},
		{"api_key", true},
		{"apikey", true},
		{"api-key", true},
		{"auth", true},
		{"auth_token", true},
		{"authorization", true},
		{"private_key", true},
		{"privatekey", true},
		{"private-key", true},
		{"access_key", true},
		{"accesskey", true},
		{"access-key", true},
		{"aws_secret_access_key", true},
		// Non-sensitive keys
		{"name", false},
		{"value", false},
		{"host", false},
		{"port", false},
		{"username", false},
		{"email", false},
		{"path", false},
		{"version", false},
		{"description", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := logging.IsSensitiveKey(tt.key)
			if got != tt.want {
				t.Errorf("IsSensitiveKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestRedactSensitiveValue(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{
			name:  "sensitive key redacted",
			key:   "password",
			value: "supersecret123",
			want:  "***",
		},
		{
			name:  "sensitive key case insensitive",
			key:   "API_TOKEN",
			value: "tok_abc123",
			want:  "***",
		},
		{
			name:  "non-sensitive key unchanged",
			key:   "hostname",
			value: "example.com",
			want:  "example.com",
		},
		{
			name:  "empty value stays empty when sensitive",
			key:   "secret",
			value: "",
			want:  "***",
		},
		{
			name:  "empty value stays empty when not sensitive",
			key:   "name",
			value: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logging.RedactSensitiveValue(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("RedactSensitiveValue(%q, %q) = %q, want %q", tt.key, tt.value, got, tt.want)
			}
		})
	}
}

func TestRedactSensitivePatterns(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "password pattern",
			input: "connecting with password=secret123 to host",
			want:  "connecting with password=*** to host",
		},
		{
			name:  "token pattern",
			input: "using token=abc123xyz for auth",
			want:  "using token=*** for auth",
		},
		{
			name:  "multiple patterns",
			input: "password=pass1 and token=tok2",
			want:  "password=*** and token=***",
		},
		{
			name:  "no sensitive patterns",
			input: "normal log message without secrets",
			want:  "normal log message without secrets",
		},
		{
			name:  "case insensitive",
			input: "PASSWORD=Secret and TOKEN=Value",
			want:  "PASSWORD=*** and TOKEN=***",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logging.RedactSensitivePatterns(tt.input)
			if got != tt.want {
				t.Errorf("RedactSensitivePatterns(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
