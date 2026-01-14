package buildkit

// This file provides unified authentication for container registries.
//
// It serves as a thin adapter layer between go-containerregistry's
// authn.DefaultKeychain and the Docker SDK authentication format.
//
// Authentication is resolved automatically from:
//   - Docker config file (~/.docker/config.json)
//   - Credential helpers (docker-credential-*)
//   - Environment variables (via credential helpers)
//   - Falls back to anonymous access if no credentials found

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/cowdogmoo/warpgate/v3/logging"
)

// ToDockerSDKAuth resolves registry credentials using go-containerregistry's
// DefaultKeychain and converts them to Docker SDK's X-Registry-Auth header format.
//
// The returned string is a base64-encoded JSON representation of the auth config,
// suitable for use in Docker API calls.
//
// If no credentials are found for the registry, returns an empty string (anonymous access).
// The registry parameter should be just the hostname (e.g., "ghcr.io", "docker.io").
func ToDockerSDKAuth(ctx context.Context, registry string) (string, error) {
	// Parse registry as a name.Registry for authentication
	ref, err := name.NewRegistry(registry, name.StrictValidation)
	if err != nil {
		return "", fmt.Errorf("invalid registry %s: %w", registry, err)
	}

	// Resolve credentials using DefaultKeychain (Docker config, credential helpers, etc.)
	authenticator, err := authn.DefaultKeychain.Resolve(ref)
	if err != nil {
		// No credentials found, use anonymous access
		logging.DebugContext(ctx, "No credentials found for registry %s, using anonymous access", registry)
		return "", nil
	}

	// Get the auth configuration
	authConfig, err := authenticator.Authorization()
	if err != nil {
		// Failed to get authorization, use anonymous access
		logging.DebugContext(ctx, "Failed to get authorization for registry %s: %v, using anonymous access", registry, err)
		return "", nil
	}

	// If no actual credentials present, return empty (anonymous)
	if authConfig.Username == "" && authConfig.Password == "" &&
		authConfig.IdentityToken == "" && authConfig.RegistryToken == "" {
		logging.DebugContext(ctx, "No credentials found for registry %s, using anonymous access", registry)
		return "", nil
	}

	// Log successful credential resolution (without exposing credentials)
	if authConfig.Username != "" {
		logging.DebugContext(ctx, "Found credentials for registry %s (username: %s)", registry, authConfig.Username)
	} else if authConfig.IdentityToken != "" || authConfig.RegistryToken != "" {
		logging.DebugContext(ctx, "Found token-based credentials for registry %s", registry)
	}

	// Convert to Docker SDK format
	dockerAuth := dockerregistry.AuthConfig{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		IdentityToken: authConfig.IdentityToken,
		RegistryToken: authConfig.RegistryToken,
		ServerAddress: registry,
	}

	// Marshal to JSON
	authJSON, err := json.Marshal(dockerAuth)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth config: %w", err)
	}

	// Encode as base64 for X-Registry-Auth header
	return base64.URLEncoding.EncodeToString(authJSON), nil
}
