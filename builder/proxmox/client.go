/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

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

package proxmox

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	pveapi "github.com/luthermonson/go-proxmox"
)

// ProxmoxClients holds the Proxmox SDK clients and configuration used by ImageBuilder.
type ProxmoxClients struct {
	// API is the underlying Proxmox VE REST client.
	API *pveapi.Client

	// Endpoint is the Proxmox API base URL (e.g., "https://pve.example.com:8006/api2/json").
	Endpoint string

	// Node is the default Proxmox node name to operate against.
	Node string
}

// ClientConfig contains configuration for creating Proxmox clients.
type ClientConfig struct {
	// Endpoint is the Proxmox API URL, including scheme and path
	// (e.g., "https://pve.example.com:8006/api2/json").
	Endpoint string

	// Node is the default Proxmox node name to operate against.
	Node string

	// APITokenID is the API token identifier in "user@realm!tokenname" form.
	// When set with APIToken, token-based auth is used (preferred).
	APITokenID string

	// APIToken is the secret value paired with APITokenID.
	APIToken string

	// Username is the PVE user for password-based auth (e.g., "root@pam").
	// Only used when APITokenID/APIToken are not provided.
	Username string

	// Password is the password for password-based auth.
	Password string

	// InsecureSkipVerify disables TLS verification for the Proxmox API.
	// Useful for clusters with self-signed certs; should not be used in
	// production against untrusted networks.
	InsecureSkipVerify bool

	// HTTPTimeout sets the underlying http.Client timeout. When zero, a
	// sensible default of 60s is used.
	HTTPTimeout time.Duration
}

// newClient is the function used to construct the Proxmox API client. Tests
// can override it to inject a fake without contacting Proxmox.
var newClient = pveapi.NewClient

// NewProxmoxClients creates a new Proxmox client with the given configuration.
// At least one of (APITokenID + APIToken) or (Username + Password) must be set.
func NewProxmoxClients(_ context.Context, cfg ClientConfig) (*ProxmoxClients, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("proxmox: Endpoint is required")
	}
	if cfg.Node == "" {
		return nil, errors.New("proxmox: Node is required")
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // opt-in via config
			},
		},
	}

	opts := []pveapi.Option{pveapi.WithHTTPClient(httpClient)}

	switch {
	case cfg.APITokenID != "" && cfg.APIToken != "":
		opts = append(opts, pveapi.WithAPIToken(cfg.APITokenID, cfg.APIToken))
	case cfg.Username != "" && cfg.Password != "":
		opts = append(opts, pveapi.WithCredentials(&pveapi.Credentials{
			Username: cfg.Username,
			Password: cfg.Password,
		}))
	default:
		return nil, errors.New("proxmox: must set either APITokenID+APIToken or Username+Password")
	}

	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	client := newClient(endpoint, opts...)

	return &ProxmoxClients{
		API:      client,
		Endpoint: endpoint,
		Node:     cfg.Node,
	}, nil
}

// GetNode returns the configured default Proxmox node.
func (c *ProxmoxClients) GetNode() string {
	return c.Node
}

// formatEndpointError adds endpoint context to API errors so users can tell
// which Proxmox cluster a failure came from when several are configured.
func (c *ProxmoxClients) formatEndpointError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s on %s: %w", action, c.Endpoint, err)
}
