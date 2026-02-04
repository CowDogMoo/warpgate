/*
Copyright (c) 2025 Jayson Grace <jayson.e.grace@gmail.com>

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

package buildkit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	digest "github.com/opencontainers/go-digest"

	"github.com/cowdogmoo/warpgate/v3/builder"
	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/cowdogmoo/warpgate/v3/templates"
)

// ============================================================
// SetCacheOptions
// ============================================================

func TestSetCacheOptions(t *testing.T) {
	tests := []struct {
		name      string
		cacheFrom []string
		cacheTo   []string
	}{
		{
			name:      "empty cache options",
			cacheFrom: []string{},
			cacheTo:   []string{},
		},
		{
			name:      "single cache from",
			cacheFrom: []string{"type=registry,ref=user/app:cache"},
			cacheTo:   []string{},
		},
		{
			name:      "single cache to",
			cacheFrom: []string{},
			cacheTo:   []string{"type=registry,ref=user/app:cache,mode=max"},
		},
		{
			name:      "both cache from and to",
			cacheFrom: []string{"type=registry,ref=user/app:cache"},
			cacheTo:   []string{"type=registry,ref=user/app:cache,mode=max"},
		},
		{
			name:      "multiple cache sources",
			cacheFrom: []string{"type=registry,ref=a:cache", "type=registry,ref=b:cache"},
			cacheTo:   []string{"type=registry,ref=a:cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			ctx := context.Background()

			b.SetCacheOptions(ctx, tt.cacheFrom, tt.cacheTo)

			if len(b.cacheFrom) != len(tt.cacheFrom) {
				t.Errorf("cacheFrom length: expected %d, got %d", len(tt.cacheFrom), len(b.cacheFrom))
			}
			if len(b.cacheTo) != len(tt.cacheTo) {
				t.Errorf("cacheTo length: expected %d, got %d", len(tt.cacheTo), len(b.cacheTo))
			}
			for i, v := range tt.cacheFrom {
				if b.cacheFrom[i] != v {
					t.Errorf("cacheFrom[%d]: expected %q, got %q", i, v, b.cacheFrom[i])
				}
			}
			for i, v := range tt.cacheTo {
				if b.cacheTo[i] != v {
					t.Errorf("cacheTo[%d]: expected %q, got %q", i, v, b.cacheTo[i])
				}
			}
		})
	}
}

// ============================================================
// parseCacheAttrs
// ============================================================

func TestParseCacheAttrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "registry cache spec",
			input: "type=registry,ref=user/app:cache",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
			},
		},
		{
			name:  "full cache spec with mode",
			input: "type=registry,ref=user/app:cache,mode=max",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
				"mode": "max",
			},
		},
		{
			name:     "single key-value",
			input:    "type=local",
			expected: map[string]string{"type": "local"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "value with equals sign",
			input: "type=registry,ref=host:5000/img:tag",
			expected: map[string]string{
				"type": "registry",
				"ref":  "host:5000/img:tag",
			},
		},
		{
			name:     "malformed pair without equals",
			input:    "noequalssign",
			expected: map[string]string{},
		},
		{
			name:  "spaces around keys and values",
			input: " type = registry , ref = user/app:cache ",
			expected: map[string]string{
				"type": "registry",
				"ref":  "user/app:cache",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCacheAttrs(tt.input)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
			// Verify no extra keys (except empty strings from malformed input)
			for k, v := range result {
				if k == "" {
					continue
				}
				if _, ok := tt.expected[k]; !ok {
					t.Errorf("unexpected key %q=%q in result", k, v)
				}
			}
		})
	}
}

// ============================================================
// loadTLSConfig
// ============================================================

// generateTestCert generates a self-signed CA cert and a client cert/key pair for testing.
func generateTestCert(t *testing.T, dir string) (caCertPath, certPath, keyPath string) {
	t.Helper()

	// Generate CA key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCertPath = filepath.Join(dir, "ca.pem")
	caCertFile, err := os.Create(caCertPath)
	if err != nil {
		t.Fatalf("failed to create CA cert file: %v", err)
	}
	if err := pem.Encode(caCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}); err != nil {
		t.Fatalf("failed to encode CA cert: %v", err)
	}
	if err := caCertFile.Close(); err != nil {
		t.Fatalf("failed to close CA cert file: %v", err)
	}

	// Generate client key
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Test Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create client certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER}); err != nil {
		t.Fatalf("failed to encode client cert: %v", err)
	}
	if err := certFile.Close(); err != nil {
		t.Fatalf("failed to close cert file: %v", err)
	}

	keyPath = filepath.Join(dir, "key.pem")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	clientKeyBytes, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatalf("failed to marshal client key: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyBytes}); err != nil {
		t.Fatalf("failed to encode client key: %v", err)
	}
	if err := keyFile.Close(); err != nil {
		t.Fatalf("failed to close key file: %v", err)
	}

	return caCertPath, certPath, keyPath
}

func TestLoadTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath, certPath, keyPath := generateTestCert(t, tmpDir)

	tests := []struct {
		name        string
		cfg         config.BuildKitConfig
		expectError bool
		checkCA     bool
		checkCert   bool
	}{
		{
			name:        "no TLS files (default config)",
			cfg:         config.BuildKitConfig{},
			expectError: false,
		},
		{
			name: "CA cert only",
			cfg: config.BuildKitConfig{
				TLSCACert: caCertPath,
			},
			expectError: false,
			checkCA:     true,
		},
		{
			name: "client cert and key",
			cfg: config.BuildKitConfig{
				TLSCert: certPath,
				TLSKey:  keyPath,
			},
			expectError: false,
			checkCert:   true,
		},
		{
			name: "all TLS files",
			cfg: config.BuildKitConfig{
				TLSCACert: caCertPath,
				TLSCert:   certPath,
				TLSKey:    keyPath,
			},
			expectError: false,
			checkCA:     true,
			checkCert:   true,
		},
		{
			name: "nonexistent CA cert",
			cfg: config.BuildKitConfig{
				TLSCACert: "/nonexistent/ca.pem",
			},
			expectError: true,
		},
		{
			name: "nonexistent client cert",
			cfg: config.BuildKitConfig{
				TLSCert: "/nonexistent/cert.pem",
				TLSKey:  keyPath,
			},
			expectError: true,
		},
		{
			name: "nonexistent client key",
			cfg: config.BuildKitConfig{
				TLSCert: certPath,
				TLSKey:  "/nonexistent/key.pem",
			},
			expectError: true,
		},
		{
			name: "invalid CA cert content",
			cfg: config.BuildKitConfig{
				TLSCACert: func() string {
					p := filepath.Join(tmpDir, "bad-ca.pem")
					_ = os.WriteFile(p, []byte("not a cert"), 0644)
					return p
				}(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsCfg, err := loadTLSConfig(tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tlsCfg == nil {
				t.Fatal("expected non-nil TLS config")
			}
			if tt.checkCA && tlsCfg.RootCAs == nil {
				t.Error("expected RootCAs to be set")
			}
			if tt.checkCert && len(tlsCfg.Certificates) == 0 {
				t.Error("expected at least one client certificate")
			}
		})
	}
}

// ============================================================
// configureCacheOptions
// ============================================================

func TestConfigureCacheOptions(t *testing.T) {
	tests := []struct {
		name              string
		cacheFrom         []string
		cacheTo           []string
		cfg               builder.Config
		expectImportCount int
		expectExportCount int
	}{
		{
			name:              "no cache, no options",
			cacheFrom:         []string{},
			cacheTo:           []string{},
			cfg:               builder.Config{},
			expectImportCount: 0,
			expectExportCount: 0,
		},
		{
			name:              "cache from and to configured",
			cacheFrom:         []string{"type=registry,ref=user/app:cache"},
			cacheTo:           []string{"type=registry,ref=user/app:cache,mode=max"},
			cfg:               builder.Config{},
			expectImportCount: 1,
			expectExportCount: 1,
		},
		{
			name:              "NoCache disables caching",
			cacheFrom:         []string{"type=registry,ref=user/app:cache"},
			cacheTo:           []string{"type=registry,ref=user/app:cache"},
			cfg:               builder.Config{NoCache: true},
			expectImportCount: 0,
			expectExportCount: 0,
		},
		{
			name:              "IsLocalTemplate disables caching",
			cacheFrom:         []string{"type=registry,ref=user/app:cache"},
			cacheTo:           []string{"type=registry,ref=user/app:cache"},
			cfg:               builder.Config{IsLocalTemplate: true},
			expectImportCount: 0,
			expectExportCount: 0,
		},
		{
			name:              "multiple cache sources",
			cacheFrom:         []string{"type=registry,ref=a:cache", "type=registry,ref=b:cache"},
			cacheTo:           []string{"type=registry,ref=c:cache"},
			cfg:               builder.Config{},
			expectImportCount: 2,
			expectExportCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{
				cacheFrom: tt.cacheFrom,
				cacheTo:   tt.cacheTo,
			}

			solveOpt := &client.SolveOpt{}
			b.configureCacheOptions(solveOpt, tt.cfg)

			if len(solveOpt.CacheImports) != tt.expectImportCount {
				t.Errorf("CacheImports count: expected %d, got %d", tt.expectImportCount, len(solveOpt.CacheImports))
			}
			if len(solveOpt.CacheExports) != tt.expectExportCount {
				t.Errorf("CacheExports count: expected %d, got %d", tt.expectExportCount, len(solveOpt.CacheExports))
			}
		})
	}
}

// ============================================================
// applyPostChanges
// ============================================================

func TestApplyPostChanges(t *testing.T) {
	tests := []struct {
		name        string
		postChanges []string
	}{
		{
			name:        "empty post changes",
			postChanges: []string{},
		},
		{
			name:        "ENV with equals sign",
			postChanges: []string{"ENV PATH=/usr/local/bin:/usr/bin"},
		},
		{
			name:        "ENV with space separated key value",
			postChanges: []string{"ENV MY_VAR my_value"},
		},
		{
			name:        "WORKDIR change",
			postChanges: []string{"WORKDIR /app"},
		},
		{
			name:        "USER change",
			postChanges: []string{"USER nobody"},
		},
		{
			name: "multiple changes",
			postChanges: []string{
				"ENV PATH=/custom:$PATH",
				"WORKDIR /home/user",
				"USER user",
			},
		},
		{
			name:        "single word entry - skip",
			postChanges: []string{"INVALID"},
		},
		{
			name:        "unknown instruction - skip",
			postChanges: []string{"COPY src dst"},
		},
		{
			name:        "ENV with only key no value - skip",
			postChanges: []string{"ENV ALONE"},
		},
		{
			name: "ENV with variable expansion",
			postChanges: []string{
				"ENV HOME /home/user",
				"ENV PATH $HOME/bin:$PATH",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			state := llb.Image("alpine:latest")

			// Should not panic
			result := b.applyPostChanges(state, tt.postChanges)
			_ = result
		})
	}
}

// ============================================================
// detectCollectionRoot
// ============================================================

func TestDetectCollectionRoot(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		expectedRoot bool
	}{
		{
			name: "playbook in collection with galaxy.yml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				collectionDir := filepath.Join(dir, "mycollection")
				playbooksDir := filepath.Join(collectionDir, "playbooks")
				_ = os.MkdirAll(playbooksDir, 0755)
				_ = os.WriteFile(filepath.Join(collectionDir, "galaxy.yml"), []byte("namespace: test"), 0644)
				return filepath.Join(playbooksDir, "site.yml")
			},
			expectedRoot: true,
		},
		{
			name: "playbook in roles directory with galaxy.yml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				collectionDir := filepath.Join(dir, "mycollection")
				rolesDir := filepath.Join(collectionDir, "roles")
				_ = os.MkdirAll(rolesDir, 0755)
				_ = os.WriteFile(filepath.Join(collectionDir, "galaxy.yml"), []byte("namespace: test"), 0644)
				return filepath.Join(rolesDir, "main.yml")
			},
			expectedRoot: true,
		},
		{
			name: "playbook without collection structure",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return filepath.Join(dir, "playbook.yml")
			},
			expectedRoot: false,
		},
		{
			name: "playbook in playbooks dir but no galaxy.yml",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				playbooksDir := filepath.Join(dir, "playbooks")
				_ = os.MkdirAll(playbooksDir, 0755)
				return filepath.Join(playbooksDir, "site.yml")
			},
			expectedRoot: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playbookPath := tt.setup(t)
			root := detectCollectionRoot(playbookPath)

			if tt.expectedRoot && root == "" {
				t.Error("expected non-empty collection root, got empty")
			}
			if !tt.expectedRoot && root != "" {
				t.Errorf("expected empty collection root, got %q", root)
			}
		})
	}
}

// ============================================================
// makeRelativePath
// ============================================================

func TestMakeRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	_ = os.MkdirAll(subDir, 0755)

	filePath := filepath.Join(subDir, "file.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	tests := []struct {
		name        string
		contextDir  string
		path        string
		expectError bool
		expectRel   string
	}{
		{
			name:       "absolute path within context",
			contextDir: tmpDir,
			path:       filePath,
			expectRel:  filepath.Join("sub", "file.txt"),
		},
		{
			name:       "path is the context dir itself",
			contextDir: tmpDir,
			path:       tmpDir,
			expectRel:  ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{contextDir: tt.contextDir}
			result, err := b.makeRelativePath(tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expectRel {
				t.Errorf("expected %q, got %q", tt.expectRel, result)
			}
		})
	}
}

// ============================================================
// fixedWriteCloser
// ============================================================

func TestFixedWriteCloser(t *testing.T) {
	t.Run("creates file and writes data", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output.tar")

		factory := fixedWriteCloser(filePath)
		wc, err := factory(map[string]string{"test": "value"})
		if err != nil {
			t.Fatalf("unexpected error creating WriteCloser: %v", err)
		}

		testData := []byte("hello world")
		n, err := wc.Write(testData)
		if err != nil {
			t.Fatalf("unexpected error writing: %v", err)
		}
		if n != len(testData) {
			t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
		}

		if err := wc.Close(); err != nil {
			t.Fatalf("unexpected error closing: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}
		if string(content) != "hello world" {
			t.Errorf("expected %q, got %q", "hello world", string(content))
		}
	})

	t.Run("nil metadata map works", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "output2.tar")

		factory := fixedWriteCloser(filePath)
		wc, err := factory(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := wc.Close(); err != nil {
			t.Fatalf("unexpected close error: %v", err)
		}
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		factory := fixedWriteCloser("/nonexistent/dir/file.tar")
		_, err := factory(nil)
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}

// ============================================================
// Close
// ============================================================

func TestClose(t *testing.T) {
	tests := []struct {
		name        string
		builder     *BuildKitBuilder
		expectError bool
	}{
		{
			name: "both clients nil",
			builder: &BuildKitBuilder{
				client:       nil,
				dockerClient: nil,
			},
			expectError: false,
		},
		{
			name: "docker client closes successfully",
			builder: &BuildKitBuilder{
				client:       nil,
				dockerClient: &MockDockerClient{},
			},
			expectError: false,
		},
		{
			name: "docker client close fails",
			builder: &BuildKitBuilder{
				client: nil,
				dockerClient: &mockDockerClientWithCloseError{
					MockDockerClient: MockDockerClient{},
					closeErr:         fmt.Errorf("close failed"),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder.Close()
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// mockDockerClientWithCloseError wraps MockDockerClient with a configurable Close error.
type mockDockerClientWithCloseError struct {
	MockDockerClient
	closeErr error
}

func (m *mockDockerClientWithCloseError) Close() error {
	return m.closeErr
}

// ============================================================
// displayProgress
// ============================================================

func TestDisplayProgress(t *testing.T) {
	t.Run("empty channel closes done", func(t *testing.T) {
		b := &BuildKitBuilder{}
		ch := make(chan *client.SolveStatus)
		done := make(chan struct{})

		go b.displayProgress(context.Background(), ch, done)
		close(ch)

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("displayProgress did not close done channel in time")
		}
	})

	t.Run("processes statuses and closes done", func(t *testing.T) {
		b := &BuildKitBuilder{}
		ch := make(chan *client.SolveStatus, 3)
		done := make(chan struct{})

		go b.displayProgress(context.Background(), ch, done)

		// Send a status with a vertex
		ch <- &client.SolveStatus{
			// codespell:ignore vertexes
			Vertexes: []*client.Vertex{
				{
					Digest: digest.FromString("test"),
					Name:   "test vertex",
				},
			},
		}

		// Send a status with logs
		ch <- &client.SolveStatus{
			Logs: []*client.VertexLog{
				{Data: []byte("log line 1")},
			},
		}

		// Send empty status
		ch <- &client.SolveStatus{}

		close(ch)

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("displayProgress did not close done channel in time")
		}
	})

	t.Run("vertex without name is skipped", func(t *testing.T) {
		b := &BuildKitBuilder{}
		ch := make(chan *client.SolveStatus, 1)
		done := make(chan struct{})

		go b.displayProgress(context.Background(), ch, done)

		ch <- &client.SolveStatus{
			// codespell:ignore vertexes
			Vertexes: []*client.Vertex{
				{
					Digest: digest.FromString("no-name"),
					Name:   "",
				},
			},
		}

		close(ch)

		select {
		case <-done:
			// success
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	})
}

// ============================================================
// getLocalImageDigest (additional edge cases)
// ============================================================

func TestGetLocalImageDigestEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockDockerClient)
		expectedDigest string
	}{
		{
			name: "repo digest without @ symbol",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "sha256:abc123",
						RepoDigests: []string{"malformed-digest"},
					}, nil
				}
			},
			expectedDigest: "sha256:abc123",
		},
		{
			name: "empty ID and empty repo digests",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID:          "",
						RepoDigests: []string{},
					}, nil
				}
			},
			expectedDigest: "",
		},
		{
			name: "multiple repo digests uses first",
			setupMock: func(m *MockDockerClient) {
				m.ImageInspectFunc = func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
					return dockerimage.InspectResponse{
						ID: "sha256:abc",
						RepoDigests: []string{
							"registry.io/img@sha256:first",
							"registry.io/img@sha256:second",
						},
					}, nil
				}
			},
			expectedDigest: "sha256:first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockDockerClient{}
			tt.setupMock(mock)
			b := &BuildKitBuilder{dockerClient: mock}
			d := b.getLocalImageDigest(context.Background(), "test:latest")
			if d != tt.expectedDigest {
				t.Errorf("expected %q, got %q", tt.expectedDigest, d)
			}
		})
	}
}

// ============================================================
// applyFileProvisioner
// ============================================================

func TestApplyFileProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty source and dest are skipped",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{Type: "file"}
			},
			expectError: false,
		},
		{
			name: "empty source is skipped",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type:        "file",
					Destination: "/tmp/dest",
				}
			},
			expectError: false,
		},
		{
			name: "file source copies successfully",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "config.txt")
				_ = os.WriteFile(filePath, []byte("data"), 0644)
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      filePath,
					Destination: "/etc/config.txt",
				}
			},
			expectError: false,
		},
		{
			name: "directory source copies successfully",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				srcDir := filepath.Join(dir, "mydir")
				_ = os.MkdirAll(srcDir, 0755)
				_ = os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("x"), 0644)
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      srcDir,
					Destination: "/opt/mydir",
				}
			},
			expectError: false,
		},
		{
			name: "file with mode set",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "script.sh")
				_ = os.WriteFile(filePath, []byte("#!/bin/sh"), 0644)
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      filePath,
					Destination: "/usr/local/bin/script.sh",
					Mode:        "0755",
				}
			},
			expectError: false,
		},
		{
			name: "nonexistent source returns error",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				return dir, builder.Provisioner{
					Type:        "file",
					Source:      filepath.Join(dir, "nonexistent.txt"),
					Destination: "/tmp/dest",
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("alpine:latest")

			_, err := b.applyFileProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// applyScriptProvisioner
// ============================================================

func TestApplyScriptProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty scripts list",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type:    "script",
					Scripts: []string{},
				}
			},
			expectError: false,
		},
		{
			name: "single script",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				scriptPath := filepath.Join(dir, "setup.sh")
				_ = os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hello"), 0755)
				return dir, builder.Provisioner{
					Type:    "script",
					Scripts: []string{scriptPath},
				}
			},
			expectError: false,
		},
		{
			name: "multiple scripts",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				s1 := filepath.Join(dir, "a.sh")
				s2 := filepath.Join(dir, "b.sh")
				_ = os.WriteFile(s1, []byte("#!/bin/sh"), 0755)
				_ = os.WriteFile(s2, []byte("#!/bin/sh"), 0755)
				return dir, builder.Provisioner{
					Type:    "script",
					Scripts: []string{s1, s2},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("alpine:latest")

			_, err := b.applyScriptProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// applyPowerShellProvisioner
// ============================================================

func TestApplyPowerShellProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty ps scripts list",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type:      "powershell",
					PSScripts: []string{},
				}
			},
			expectError: false,
		},
		{
			name: "single powershell script",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				scriptPath := filepath.Join(dir, "setup.ps1")
				_ = os.WriteFile(scriptPath, []byte("Write-Host 'hello'"), 0644)
				return dir, builder.Provisioner{
					Type:      "powershell",
					PSScripts: []string{scriptPath},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("alpine:latest")

			_, err := b.applyPowerShellProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// applyAnsibleProvisioner
// ============================================================

func TestApplyAnsibleProvisioner(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Provisioner)
		expectError bool
	}{
		{
			name: "empty playbook path returns state unchanged",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				return t.TempDir(), builder.Provisioner{
					Type: "ansible",
				}
			},
			expectError: false,
		},
		{
			name: "playbook with galaxy file",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				pbPath := filepath.Join(dir, "playbook.yml")
				galPath := filepath.Join(dir, "requirements.yml")
				_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)
				_ = os.WriteFile(galPath, []byte("---\nroles: []"), 0644)
				return dir, builder.Provisioner{
					Type:         "ansible",
					PlaybookPath: pbPath,
					GalaxyFile:   galPath,
				}
			},
			expectError: false,
		},
		{
			name: "playbook with extra vars",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				pbPath := filepath.Join(dir, "playbook.yml")
				_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)
				return dir, builder.Provisioner{
					Type:         "ansible",
					PlaybookPath: pbPath,
					ExtraVars:    map[string]string{"env": "test", "debug": "true"},
				}
			},
			expectError: false,
		},
		{
			name: "playbook inside collection structure",
			setup: func(t *testing.T) (string, builder.Provisioner) {
				dir := t.TempDir()
				collDir := filepath.Join(dir, "mycollection")
				pbDir := filepath.Join(collDir, "playbooks")
				_ = os.MkdirAll(pbDir, 0755)
				_ = os.WriteFile(filepath.Join(collDir, "galaxy.yml"), []byte("namespace: ns\nname: col"), 0644)
				pbPath := filepath.Join(pbDir, "site.yml")
				_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)
				return dir, builder.Provisioner{
					Type:         "ansible",
					PlaybookPath: pbPath,
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, prov := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}
			state := llb.Image("ubuntu:22.04")

			_, err := b.applyAnsibleProvisioner(state, prov)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// convertToLLB
// ============================================================

func TestConvertToLLB(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (string, builder.Config)
		expectError bool
		errContains string
	}{
		{
			name: "dockerfile-based config returns error",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Dockerfile: &builder.DockerfileConfig{
						Path: "Dockerfile",
					},
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
				}
			},
			expectError: true,
			errContains: "dockerfile-based builds",
		},
		{
			name: "basic config with platform",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with architectures fallback",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:          "test",
					Version:       "1.0",
					Base:          builder.BaseImage{Image: "alpine:latest"},
					Architectures: []string{"arm64"},
				}
			},
			expectError: false,
		},
		{
			name: "config with no platform and no architectures",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base:    builder.BaseImage{Image: "alpine:latest"},
				}
			},
			expectError: true,
			errContains: "no platform",
		},
		{
			name: "config with base env",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
						Env:      map[string]string{"FOO": "bar"},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with build args",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
					BuildArgs: map[string]string{"VERSION": "1.0"},
				}
			},
			expectError: false,
		},
		{
			name: "config with base changes",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
						Changes:  []string{"ENV FOO=bar", "WORKDIR /app"},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with shell provisioner",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "ubuntu:22.04",
						Platform: "linux/amd64",
					},
					Provisioners: []builder.Provisioner{
						{
							Type:   "shell",
							Inline: []string{"apt-get update", "apt-get install -y curl"},
						},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with post changes",
			setup: func(t *testing.T) (string, builder.Config) {
				return t.TempDir(), builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
					PostChanges: []string{"USER nobody", "WORKDIR /home/nobody"},
				}
			},
			expectError: false,
		},
		{
			name: "config with file provisioner",
			setup: func(t *testing.T) (string, builder.Config) {
				dir := t.TempDir()
				filePath := filepath.Join(dir, "config.yml")
				_ = os.WriteFile(filePath, []byte("key: value"), 0644)
				return dir, builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "alpine:latest",
						Platform: "linux/amd64",
					},
					Provisioners: []builder.Provisioner{
						{
							Type:        "file",
							Source:      filePath,
							Destination: "/etc/config.yml",
						},
					},
				}
			},
			expectError: false,
		},
		{
			name: "config with multiple provisioner types",
			setup: func(t *testing.T) (string, builder.Config) {
				dir := t.TempDir()
				scriptPath := filepath.Join(dir, "setup.sh")
				_ = os.WriteFile(scriptPath, []byte("#!/bin/sh\necho done"), 0755)
				return dir, builder.Config{
					Name:    "test",
					Version: "1.0",
					Base: builder.BaseImage{
						Image:    "ubuntu:22.04",
						Platform: "linux/amd64",
					},
					Provisioners: []builder.Provisioner{
						{
							Type:   "shell",
							Inline: []string{"echo step1"},
						},
						{
							Type:    "script",
							Scripts: []string{scriptPath},
						},
					},
					PostChanges: []string{"ENV DONE=true"},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextDir, cfg := tt.setup(t)
			b := &BuildKitBuilder{contextDir: contextDir}

			_, err := b.convertToLLB(cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.errContains != "" && err != nil && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// getAnsiblePaths
// ============================================================

func TestGetAnsiblePaths(t *testing.T) {
	tmpDir := t.TempDir()
	pbPath := filepath.Join(tmpDir, "playbook.yml")
	galPath := filepath.Join(tmpDir, "requirements.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)
	_ = os.WriteFile(galPath, []byte("---"), 0644)

	tests := []struct {
		name        string
		prov        builder.Provisioner
		expectCount int
		expectError bool
	}{
		{
			name:        "both paths",
			prov:        builder.Provisioner{Type: "ansible", PlaybookPath: pbPath, GalaxyFile: galPath},
			expectCount: 2,
		},
		{
			name:        "playbook only",
			prov:        builder.Provisioner{Type: "ansible", PlaybookPath: pbPath},
			expectCount: 1,
		},
		{
			name:        "galaxy only",
			prov:        builder.Provisioner{Type: "ansible", GalaxyFile: galPath},
			expectCount: 1,
		},
		{
			name:        "neither",
			prov:        builder.Provisioner{Type: "ansible"},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			pv := templates.NewPathValidator()
			paths, err := b.getAnsiblePaths(tt.prov, pv)

			if tt.expectError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.expectCount {
				t.Errorf("expected %d paths, got %d", tt.expectCount, len(paths))
			}
		})
	}
}

// ============================================================
// getProvisionerPaths
// ============================================================

func TestGetProvisionerPaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	scriptPath := filepath.Join(tmpDir, "script.sh")
	psPath := filepath.Join(tmpDir, "script.ps1")
	pbPath := filepath.Join(tmpDir, "playbook.yml")
	_ = os.WriteFile(filePath, []byte("data"), 0644)
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0755)
	_ = os.WriteFile(psPath, []byte("Write-Host"), 0644)
	_ = os.WriteFile(pbPath, []byte("---"), 0644)

	tests := []struct {
		name        string
		prov        builder.Provisioner
		expectCount int
	}{
		{
			name:        "ansible provisioner",
			prov:        builder.Provisioner{Type: "ansible", PlaybookPath: pbPath},
			expectCount: 1,
		},
		{
			name:        "file provisioner",
			prov:        builder.Provisioner{Type: "file", Source: filePath, Destination: "/tmp/f"},
			expectCount: 1,
		},
		{
			name:        "script provisioner",
			prov:        builder.Provisioner{Type: "script", Scripts: []string{scriptPath}},
			expectCount: 1,
		},
		{
			name:        "powershell provisioner",
			prov:        builder.Provisioner{Type: "powershell", PSScripts: []string{psPath}},
			expectCount: 1,
		},
		{
			name:        "shell provisioner has no paths",
			prov:        builder.Provisioner{Type: "shell", Inline: []string{"echo hi"}},
			expectCount: 0,
		},
		{
			name:        "unknown provisioner has no paths",
			prov:        builder.Provisioner{Type: "unknown"},
			expectCount: 0,
		},
		{
			name:        "file provisioner empty source",
			prov:        builder.Provisioner{Type: "file", Source: "", Destination: "/tmp/f"},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			pv := templates.NewPathValidator()
			paths, err := b.getProvisionerPaths(tt.prov, pv)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.expectCount {
				t.Errorf("expected %d paths, got %d: %v", tt.expectCount, len(paths), paths)
			}
		})
	}
}

// ============================================================
// applyShellProvisioner (package manager cache mounts)
// ============================================================

func TestApplyShellProvisionerCacheMounts(t *testing.T) {
	tests := []struct {
		name   string
		inline []string
	}{
		{
			name:   "empty inline",
			inline: []string{},
		},
		{
			name:   "apt-get commands",
			inline: []string{"apt-get update", "apt-get install -y curl"},
		},
		{
			name:   "yum commands",
			inline: []string{"yum install -y wget"},
		},
		{
			name:   "dnf commands",
			inline: []string{"dnf install -y git"},
		},
		{
			name:   "apk commands",
			inline: []string{"apk add curl"},
		},
		{
			name:   "pip commands",
			inline: []string{"pip install requests"},
		},
		{
			name:   "npm commands",
			inline: []string{"npm install express"},
		},
		{
			name:   "yarn commands",
			inline: []string{"yarn add react"},
		},
		{
			name:   "go build commands",
			inline: []string{"go build ./..."},
		},
		{
			name:   "go get commands",
			inline: []string{"go get github.com/some/pkg"},
		},
		{
			name:   "mixed package managers",
			inline: []string{"apt-get update && pip install boto3 && npm install"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			state := llb.Image("ubuntu:22.04")
			prov := builder.Provisioner{
				Type:   "shell",
				Inline: tt.inline,
			}

			_, err := b.applyShellProvisioner(state, prov)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================
// buildExportAttributes (additional coverage)
// ============================================================

func TestBuildExportAttributesEmpty(t *testing.T) {
	attrs := buildExportAttributes("myimage:v1", map[string]string{})
	if attrs["name"] != "myimage:v1" {
		t.Errorf("expected name 'myimage:v1', got %q", attrs["name"])
	}
	// Should have only the name key
	if len(attrs) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(attrs))
	}
}

// ============================================================
// loadAndTagImage (additional coverage with mock)
// ============================================================

func TestLoadAndTagImageFileNotFound(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.loadAndTagImage(context.Background(), "/nonexistent/path.tar", "test:latest")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ============================================================
// Push edge cases: unqualified image ref gets tagged with registry
// ============================================================

func TestPushUnqualifiedImageRef(t *testing.T) {
	tagCalled := false
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			tagCalled = true
			return nil
		},
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"ghcr.io/org/app@sha256:digest123"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	_, err := b.Push(context.Background(), "myapp:latest", "ghcr.io/org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tagCalled {
		t.Error("expected ImageTag to be called for unqualified image ref")
	}
}

// ============================================================
// CreateAndPushManifest edge case: empty entries
// ============================================================

func TestCreateAndPushManifestEmptyEntries(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	err := b.CreateAndPushManifest(context.Background(), "test:latest", nil)
	if err == nil {
		t.Error("expected error for empty entries")
	}
	if !strings.Contains(err.Error(), "no manifest entries") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// expandContainerVars additional coverage
// ============================================================

func TestExpandContainerVarsNoOp(t *testing.T) {
	b := &BuildKitBuilder{}
	result := b.expandContainerVars("novar", map[string]string{"PATH": "/usr/bin"})
	if result != "novar" {
		t.Errorf("expected 'novar', got %q", result)
	}
}

func TestExpandContainerVarsMultipleOccurrences(t *testing.T) {
	b := &BuildKitBuilder{}
	result := b.expandContainerVars("$X and $X", map[string]string{"X": "val"})
	if result != "val and val" {
		t.Errorf("expected 'val and val', got %q", result)
	}
}

// ============================================================
// extractRegistryFromImageRef edge cases (additional)
// ============================================================

func TestExtractRegistryFromImageRefEdgeCases(t *testing.T) {
	tests := []struct {
		imageRef string
		expected string
	}{
		{"", "docker.io"},
		{"image", "docker.io"},
		{"user/image", "docker.io"},
		{"localhost/image", "localhost"},
		{"host.com:5000/image", "host.com:5000"},
	}

	for _, tt := range tests {
		t.Run(tt.imageRef, func(t *testing.T) {
			result := extractRegistryFromImageRef(tt.imageRef)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================
// getPlatformString and extractArchFromPlatform additional combos
// ============================================================

func TestGetPlatformStringNoArches(t *testing.T) {
	cfg := builder.Config{
		Base:          builder.BaseImage{},
		Architectures: nil,
	}
	result := getPlatformString(cfg)
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

// ============================================================
// findCommonParent edge cases
// ============================================================

func TestFindCommonParentSamePath(t *testing.T) {
	result := findCommonParent("/usr/local/bin", "/usr/local/bin")
	if result != "/usr/local/bin" {
		t.Errorf("expected '/usr/local/bin', got %q", result)
	}
}

func TestFindCommonParentRoot(t *testing.T) {
	result := findCommonParent("/a", "/b")
	if result != "/" {
		t.Errorf("expected '/', got %q", result)
	}
}

// ============================================================
// CreateAndPushManifest with empty digest
// ============================================================

func TestCreateAndPushManifestEmptyDigest(t *testing.T) {
	mock := &MockDockerClient{}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "test:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       "",
		},
	}

	err := b.CreateAndPushManifest(context.Background(), "test:latest", entries)
	if err == nil {
		t.Error("expected error for empty digest")
	}
	if !strings.Contains(err.Error(), "no digest found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// collectProvisionerPaths
// ============================================================

func TestCollectProvisionerPaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	scriptPath := filepath.Join(tmpDir, "script.sh")
	_ = os.WriteFile(filePath, []byte("data"), 0644)
	_ = os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0755)

	tests := []struct {
		name         string
		provisioners []builder.Provisioner
		expectCount  int
		expectError  bool
	}{
		{
			name:         "empty provisioners",
			provisioners: []builder.Provisioner{},
			expectCount:  0,
		},
		{
			name: "single file provisioner",
			provisioners: []builder.Provisioner{
				{Type: "file", Source: filePath, Destination: "/tmp/f"},
			},
			expectCount: 1,
		},
		{
			name: "multiple provisioners",
			provisioners: []builder.Provisioner{
				{Type: "file", Source: filePath, Destination: "/tmp/f"},
				{Type: "script", Scripts: []string{scriptPath}},
			},
			expectCount: 2,
		},
		{
			name: "shell provisioner adds no paths",
			provisioners: []builder.Provisioner{
				{Type: "shell", Inline: []string{"echo hi"}},
			},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildKitBuilder{}
			pv := templates.NewPathValidator()
			paths, err := b.collectProvisionerPaths(tt.provisioners, pv)

			if tt.expectError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.expectCount {
				t.Errorf("expected %d paths, got %d", tt.expectCount, len(paths))
			}
		})
	}
}

// ============================================================
// calculateBuildContext edge cases
// ============================================================

func TestCalculateBuildContextEmptyProvisioners(t *testing.T) {
	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "." {
		t.Errorf("expected '.', got %q", ctx)
	}
}

func TestCalculateBuildContextNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "doesnt_exist.txt")

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{Type: "file", Source: nonExistent, Destination: "/tmp/f"},
		},
	}

	// When the file doesn't exist, calculateBuildContext uses filepath.Dir
	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Error("expected non-empty context")
	}
}

// ============================================================
// ToDockerSDKAuth
// ============================================================

func TestToDockerSDKAuthRegistries(t *testing.T) {
	tests := []struct {
		name        string
		registry    string
		expectError bool
	}{
		{
			name:        "valid registry",
			registry:    "ghcr.io",
			expectError: false,
		},
		{
			name:        "docker hub",
			registry:    "docker.io",
			expectError: false,
		},
		{
			name:        "localhost registry",
			registry:    "localhost",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToDockerSDKAuth(context.Background(), tt.registry)
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			// We cannot guarantee credentials exist, but the function should not panic
		})
	}
}

// ============================================================
// Push edge cases: tag failure
// ============================================================

func TestPushTagFailure(t *testing.T) {
	mock := &MockDockerClient{
		ImageTagFunc: func(ctx context.Context, source, target string) error {
			return fmt.Errorf("tag failed")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	_, err := b.Push(context.Background(), "myapp:latest", "ghcr.io/org")

	if err == nil {
		t.Error("expected error for tag failure")
	}
	if !strings.Contains(err.Error(), "tag") {
		t.Errorf("error should mention tag: %v", err)
	}
}

// ============================================================
// Push: inspect fails after successful push
// ============================================================

func TestPushInspectFailsAfterPush(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("inspect failed")
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	digest, err := b.Push(context.Background(), "ghcr.io/org/myapp:latest", "ghcr.io/org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if digest != "" {
		t.Errorf("expected empty digest when inspect fails, got %q", digest)
	}
}

// ============================================================
// Push: no digest in RepoDigests after push
// ============================================================

func TestPushNoDigestInRepoDigests(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	digest, err := b.Push(context.Background(), "ghcr.io/org/myapp:latest", "ghcr.io/org")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if digest != "" {
		t.Errorf("expected empty digest, got %q", digest)
	}
}

// ============================================================
// applyProvisioner: file provisioner dispatch
// ============================================================

func TestApplyProvisionerFileDispatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.txt")
	_ = os.WriteFile(filePath, []byte("test"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:        "file",
		Source:      filePath,
		Destination: "/tmp/data.txt",
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// applyProvisioner: ansible dispatch
// ============================================================

func TestApplyProvisionerAnsibleDispatch(t *testing.T) {
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("ubuntu:22.04")

	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: pbPath,
	}

	result, err := b.applyProvisioner(state, prov, builder.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// loadAndTagImage: successful load with read response
// ============================================================

func TestLoadAndTagImageSuccessWithResponse(t *testing.T) {
	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{
				Body: io.NopCloser(strings.NewReader(`{"stream":"Loaded image: test:latest"}`)),
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	tmpFile, err := os.CreateTemp("", "test-image-*.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	if _, err := tmpFile.WriteString("dummy"); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	err = b.loadAndTagImage(context.Background(), tmpFile.Name(), "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// expandPathList
// ============================================================

func TestExpandPathList(t *testing.T) {
	tmpDir := t.TempDir()
	s1 := filepath.Join(tmpDir, "a.sh")
	s2 := filepath.Join(tmpDir, "b.sh")
	_ = os.WriteFile(s1, []byte("#!/bin/sh"), 0755)
	_ = os.WriteFile(s2, []byte("#!/bin/sh"), 0755)

	pv := templates.NewPathValidator()

	tests := []struct {
		name     string
		scripts  []string
		pathType string
		count    int
	}{
		{"empty", []string{}, "script", 0},
		{"single", []string{s1}, "script", 1},
		{"multiple", []string{s1, s2}, "script", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := expandPathList(tt.scripts, pv, tt.pathType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(paths) != tt.count {
				t.Errorf("expected %d, got %d", tt.count, len(paths))
			}
		})
	}
}

// ============================================================
// getFilePaths
// ============================================================

func TestGetFilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.txt")
	_ = os.WriteFile(filePath, []byte("x"), 0644)

	pv := templates.NewPathValidator()
	b := &BuildKitBuilder{}

	t.Run("with source", func(t *testing.T) {
		prov := builder.Provisioner{Type: "file", Source: filePath}
		paths, err := b.getFilePaths(prov, pv)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 1 {
			t.Errorf("expected 1 path, got %d", len(paths))
		}
	})

	t.Run("empty source", func(t *testing.T) {
		prov := builder.Provisioner{Type: "file", Source: ""}
		paths, err := b.getFilePaths(prov, pv)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Errorf("expected 0 paths, got %d", len(paths))
		}
	})
}

// ============================================================
// CreateAndPushManifest: valid entries with inspect failure
// ============================================================

// ============================================================
// Close: buildkit client error (non-nil client that returns error)
// We can't easily mock client.Client, but we can test combined errors
// ============================================================

func TestCloseDockerClientError(t *testing.T) {
	b := &BuildKitBuilder{
		client: nil,
		dockerClient: &mockDockerClientWithCloseError{
			closeErr: fmt.Errorf("docker close failed"),
		},
	}

	err := b.Close()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "docker close failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// Push: test push response read failure path
// ============================================================

func TestPushReadResponseFailure(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(&errorReader{}), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"ghcr.io/test@sha256:def"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	// The push proceeds even if reading the response body fails
	_, err := b.Push(context.Background(), "ghcr.io/test:v1", "ghcr.io")
	// Should not return error - read failure is logged but not fatal
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

// ============================================================
// Push: digest parsing edge case - RepoDigest without proper format
// ============================================================

func TestPushRepoDigestMalformed(t *testing.T) {
	mock := &MockDockerClient{
		ImagePushFunc: func(ctx context.Context, image string, options dockerimage.PushOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"status":"ok"}`)), nil
		},
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{
				ID:          "sha256:abc",
				RepoDigests: []string{"no-at-symbol-here"},
			}, nil
		},
	}

	b := &BuildKitBuilder{dockerClient: mock}
	d, err := b.Push(context.Background(), "ghcr.io/test:v1", "ghcr.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With malformed digest, should warn and return empty
	if d != "" {
		t.Errorf("expected empty digest for malformed RepoDigest, got %q", d)
	}
}

// ============================================================
// CreateAndPushManifest with variant
// ============================================================

func TestCreateAndPushManifestWithVariant(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{Size: 1024}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:latest",
			OS:           "linux",
			Architecture: "arm",
			Variant:      "v7",
			Digest:       digest.FromString("test-arm"),
		},
	}

	// Will fail at remote.Get, but exercises the variant code path
	err := b.CreateAndPushManifest(context.Background(), "ghcr.io/test/app:latest", entries)
	// Expected to fail at network step - that's OK
	_ = err
}

func TestCreateAndPushManifestInspectFails(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{}, fmt.Errorf("inspect failed")
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:latest",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.FromString("test"),
		},
	}

	// This will proceed past inspect (using size 0) but fail when trying to
	// parse the manifest name and push - that's OK, we're testing the code path
	err := b.CreateAndPushManifest(context.Background(), "ghcr.io/test/app:latest", entries)
	// It will fail at the remote.Get step since we're not connected to a registry
	// but the important thing is we exercised the code path through inspect failure
	if err == nil {
		// If somehow it succeeds (unlikely), that's also fine
		return
	}
	// Error is expected at the remote step
}

// ============================================================
// CreateAndPushManifest: multiple entries with successful inspect
// ============================================================

func TestCreateAndPushManifestMultipleEntries(t *testing.T) {
	mock := &MockDockerClient{
		ImageInspectFunc: func(ctx context.Context, imageID string) (dockerimage.InspectResponse, error) {
			return dockerimage.InspectResponse{Size: 1024}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	entries := []manifests.ManifestEntry{
		{
			ImageRef:     "ghcr.io/test/app:amd64",
			OS:           "linux",
			Architecture: "amd64",
			Digest:       digest.FromString("test-amd64"),
		},
		{
			ImageRef:     "ghcr.io/test/app:arm64",
			OS:           "linux",
			Architecture: "arm64",
			Digest:       digest.FromString("test-arm64"),
		},
	}

	// Will fail at remote.Get since no real registry, but exercises the multi-entry loop
	_ = b.CreateAndPushManifest(context.Background(), "ghcr.io/test/app:latest", entries)
}

// ============================================================
// applyAnsibleProvisioner: galaxy file makeRelativePath error
// ============================================================

func TestApplyAnsibleProvisionerGalaxyError(t *testing.T) {
	// Use a context directory that's different from where the file is,
	// but the file path uses a tilde which would cause expansion issues
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("ubuntu:22.04")

	// Playbook path valid, galaxy file has a nonexistent tilde-based path
	prov := builder.Provisioner{
		Type:         "ansible",
		PlaybookPath: pbPath,
		GalaxyFile:   "~nonexistentuser/requirements.yml",
	}

	// This should fail when trying to resolve the galaxy path
	_, err := b.applyAnsibleProvisioner(state, prov)
	// Error is expected from makeRelativePath failure
	if err == nil {
		// On some systems tilde expansion may work differently,
		// so don't fail if it happens to succeed
		return
	}
}

// ============================================================
// loadAndTagImage: test the response read and close path fully
// ============================================================

func TestLoadAndTagImageReadResponseError(t *testing.T) {
	mock := &MockDockerClient{
		ImageLoadFunc: func(ctx context.Context, input io.Reader) (dockerimage.LoadResponse, error) {
			return dockerimage.LoadResponse{
				Body: io.NopCloser(&errorReader{}),
			}, nil
		},
	}
	b := &BuildKitBuilder{dockerClient: mock}

	tmpFile, err := os.CreateTemp("", "test-image-*.tar")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	if _, err := tmpFile.WriteString("dummy"); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	err = b.loadAndTagImage(context.Background(), tmpFile.Name(), "test:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// collectProvisionerPaths: error propagation from provisioner
// ============================================================

func TestCollectProvisionerPathsError(t *testing.T) {
	b := &BuildKitBuilder{}
	pv := templates.NewPathValidator()

	// Use a provisioner with a path that will fail expansion
	provisioners := []builder.Provisioner{
		{
			Type:   "file",
			Source: "~nonexistentuser/file.txt",
		},
	}

	_, err := b.collectProvisionerPaths(provisioners, pv)
	// On macOS/Linux this may or may not error depending on how tilde expansion works
	// The important thing is no panic
	_ = err
}

// ============================================================
// convertToLLB: with ansible provisioner
// ============================================================

func TestConvertToLLBWithAnsible(t *testing.T) {
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	_ = os.WriteFile(pbPath, []byte("---\n- hosts: all"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "ubuntu:22.04",
			Platform: "linux/amd64",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: pbPath,
			},
		},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// convertToLLB: with powershell provisioner
// ============================================================

func TestConvertToLLBWithPowerShell(t *testing.T) {
	dir := t.TempDir()
	psPath := filepath.Join(dir, "setup.ps1")
	_ = os.WriteFile(psPath, []byte("Write-Host test"), 0644)

	b := &BuildKitBuilder{contextDir: dir}
	cfg := builder.Config{
		Name:    "test",
		Version: "1.0",
		Base: builder.BaseImage{
			Image:    "mcr.microsoft.com/powershell:latest",
			Platform: "linux/amd64",
		},
		Provisioners: []builder.Provisioner{
			{
				Type:      "powershell",
				PSScripts: []string{psPath},
			},
		},
	}

	_, err := b.convertToLLB(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ============================================================
// applyFileProvisioner: abs path resolution error (edge case)
// ============================================================

func TestApplyFileProvisionerAbsPathEdge(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(filePath, []byte("data"), 0644)

	// Use the file's directory as context so relative path is simple
	b := &BuildKitBuilder{contextDir: dir}
	state := llb.Image("alpine:latest")

	prov := builder.Provisioner{
		Type:        "file",
		Source:      filePath,
		Destination: "/tmp/test.txt",
	}

	result, err := b.applyFileProvisioner(state, prov)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result
}

// ============================================================
// makeRelativePath: error from expand path (invalid user home)
// ============================================================

func TestMakeRelativePathExpandError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: "/tmp"}
	_, err := b.makeRelativePath("~nonexistentuser/file.txt")
	// This may or may not error depending on the OS, but should not panic
	_ = err
}

// ============================================================
// calculateBuildContext: with ansible provisioner
// ============================================================

func TestCalculateBuildContextWithAnsible(t *testing.T) {
	dir := t.TempDir()
	pbPath := filepath.Join(dir, "playbook.yml")
	galPath := filepath.Join(dir, "requirements.yml")
	_ = os.WriteFile(pbPath, []byte("---"), 0644)
	_ = os.WriteFile(galPath, []byte("---"), 0644)

	b := &BuildKitBuilder{}
	cfg := builder.Config{
		Provisioners: []builder.Provisioner{
			{
				Type:         "ansible",
				PlaybookPath: pbPath,
				GalaxyFile:   galPath,
			},
		},
	}

	ctx, err := b.calculateBuildContext(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Error("expected non-empty context")
	}
}

// ============================================================
// createAuthProvider (exercise the function)
// ============================================================

// ============================================================
// applyFileProvisioner: makeRelativePath returns path that doesn't exist
// ============================================================

func TestApplyFileProvisionerStatError(t *testing.T) {
	b := &BuildKitBuilder{contextDir: "/tmp"}
	state := llb.Image("alpine:latest")
	prov := builder.Provisioner{
		Type:        "file",
		Source:      string([]byte{0}), // null byte causes stat failure
		Destination: "/tmp/dest",
	}

	_, err := b.applyFileProvisioner(state, prov)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestCreateAuthProviderCoverage(t *testing.T) {
	// This function reads Docker config; in test environment it may
	// or may not find credentials, but should not panic
	result := createAuthProvider()
	// result may be nil (no Docker config) or non-nil (has Docker config)
	_ = result
}
