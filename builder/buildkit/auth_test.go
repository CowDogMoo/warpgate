package buildkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

// TestToDockerSDKAuth tests the conversion from DefaultKeychain to Docker SDK format
func TestToDockerSDKAuth(t *testing.T) {
	tests := []struct {
		name          string
		registry      string
		wantAnonymous bool
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid registry with Docker config",
			registry:      "ghcr.io",
			wantAnonymous: false, // Depends on local Docker config
			wantErr:       false,
		},
		{
			name:          "docker hub",
			registry:      "docker.io",
			wantAnonymous: false, // Depends on local Docker config
			wantErr:       false,
		},
		{
			name:          "localhost registry",
			registry:      "localhost:5000",
			wantAnonymous: false, // Depends on local Docker config
			wantErr:       false,
		},
		{
			name:        "invalid registry with special chars",
			registry:    "not@valid",
			wantErr:     true,
			errContains: "invalid registry",
		},
		{
			name:        "empty registry",
			registry:    "",
			wantErr:     true,
			errContains: "invalid registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := ToDockerSDKAuth(ctx, tt.registry)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ToDockerSDKAuth() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ToDockerSDKAuth() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ToDockerSDKAuth() unexpected error = %v", err)
				return
			}

			// If we got an auth string, verify it's valid base64-encoded JSON
			if got != "" {
				// Decode base64
				decoded, err := base64.URLEncoding.DecodeString(got)
				if err != nil {
					t.Errorf("ToDockerSDKAuth() returned invalid base64: %v", err)
					return
				}

				// Verify it's valid JSON and has expected structure
				var authConfig dockerregistry.AuthConfig
				if err := json.Unmarshal(decoded, &authConfig); err != nil {
					t.Errorf("ToDockerSDKAuth() returned invalid JSON: %v", err)
					return
				}

				// Should have either username/password or tokens
				hasBasicAuth := authConfig.Username != "" && authConfig.Password != ""
				hasTokenAuth := authConfig.IdentityToken != "" || authConfig.RegistryToken != ""

				if !hasBasicAuth && !hasTokenAuth {
					t.Errorf("ToDockerSDKAuth() returned auth config without credentials")
				}

				t.Logf("Successfully resolved credentials for %s (method: %s)",
					tt.registry, getAuthMethod(authConfig))
			} else if !tt.wantAnonymous {
				t.Logf("Note: No credentials found for %s (anonymous access)", tt.registry)
			}
		})
	}
}

// TestDefaultKeychainIntegration verifies authn.DefaultKeychain behavior
func TestDefaultKeychainIntegration(t *testing.T) {
	tests := []struct {
		name     string
		registry string
	}{
		{
			name:     "docker hub",
			registry: "docker.io",
		},
		{
			name:     "github container registry",
			registry: "ghcr.io",
		},
		{
			name:     "localhost",
			registry: "localhost:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies we can interact with DefaultKeychain correctly
			// Parse registry as name.Registry for authentication
			reg, err := name.NewRegistry(tt.registry, name.StrictValidation)
			if err != nil {
				t.Logf("Failed to parse registry %s: %v", tt.registry, err)
				return
			}

			authenticator, err := authn.DefaultKeychain.Resolve(reg)
			if err != nil {
				t.Logf("No credentials found for %s (expected for registries without login): %v",
					tt.registry, err)
				return
			}

			// Get authorization
			auth, err := authenticator.Authorization()
			if err != nil {
				t.Logf("Failed to get authorization for %s: %v", tt.registry, err)
				return
			}

			t.Logf("Successfully resolved auth for %s (has username: %v, has token: %v)",
				tt.registry,
				auth.Username != "",
				auth.IdentityToken != "" || auth.RegistryToken != "")
		})
	}
}

// getAuthMethod returns a human-readable string describing the auth method
func getAuthMethod(auth dockerregistry.AuthConfig) string {
	if auth.Username != "" && auth.Password != "" {
		return "username/password"
	}
	if auth.IdentityToken != "" {
		return "identity token"
	}
	if auth.RegistryToken != "" {
		return "registry token"
	}
	return "unknown"
}

// TestCreateAuthProvider tests the session auth provider creation for BuildKit
func TestCreateAuthProvider(t *testing.T) {
	attachables := createAuthProvider()

	if attachables != nil {
		if len(attachables) != 1 {
			t.Errorf("createAuthProvider() returned %d attachables, expected 1", len(attachables))
		}
		t.Logf("createAuthProvider() successfully created auth provider from Docker config")
	} else {
		t.Logf("createAuthProvider() returned nil (no Docker config available)")
	}
}
