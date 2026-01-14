// Package logging provides credential and sensitive data redaction utilities.
package logging

import (
	"net/url"
	"regexp"
	"strings"
)

// sensitiveKeyPatterns contains patterns that indicate a key holds sensitive data.
var sensitiveKeyPatterns = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"credential",
	"api_key",
	"apikey",
	"api-key",
	"auth",
	"private_key",
	"privatekey",
	"private-key",
	"access_key",
	"accesskey",
	"access-key",
}

// sensitiveValuePattern matches common sensitive patterns in values.
var sensitiveValuePattern = regexp.MustCompile(`(?i)(password|token|secret|key|credential|auth)=\S+`)

// RedactURL removes embedded credentials from URLs.
// For example: https://user:pass@host.com -> https://***:***@host.com
// If the URL cannot be parsed, it returns the original string with any
// obvious credentials redacted using pattern matching.
func RedactURL(rawURL string) string {
	if rawURL == "" {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		// Fallback to regex-based redaction for malformed URLs
		return redactURLFallback(rawURL)
	}

	// Check if URL has user info (credentials)
	if parsed.User == nil {
		return rawURL
	}

	username := parsed.User.Username()
	_, hasPassword := parsed.User.Password()

	if !hasPassword && username == "" {
		return rawURL
	}

	// Build redacted URL manually to avoid URL encoding of asterisks
	var redactedUserInfo string
	if hasPassword {
		redactedUserInfo = "***:***"
	} else {
		redactedUserInfo = "***"
	}

	// Reconstruct the URL with redacted credentials
	result := parsed.Scheme + "://" + redactedUserInfo + "@" + parsed.Host
	if parsed.Path != "" {
		result += parsed.Path
	}
	if parsed.RawQuery != "" {
		result += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		result += "#" + parsed.Fragment
	}

	return result
}

// redactURLFallback uses regex to redact credentials when URL parsing fails.
func redactURLFallback(rawURL string) string {
	// Match user:pass@host or token@host patterns
	credentialPattern := regexp.MustCompile(`://([^@/]+)@`)
	return credentialPattern.ReplaceAllString(rawURL, "://***@")
}

// IsSensitiveKey returns true if the key name matches known sensitive patterns.
// The check is case-insensitive.
func IsSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}
	return false
}

// RedactSensitiveValue returns a redacted version of the value if the key
// is sensitive, otherwise returns the original value.
func RedactSensitiveValue(key, value string) string {
	if IsSensitiveKey(key) {
		return "***"
	}
	return value
}

// RedactSensitivePatterns redacts known sensitive patterns from a string.
// For example: "password=secret123" -> "password=***"
func RedactSensitivePatterns(input string) string {
	return sensitiveValuePattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=***"
		}
		return match
	})
}
