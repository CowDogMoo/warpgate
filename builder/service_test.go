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

package builder

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/manifests"
	"github.com/opencontainers/go-digest"
)

// mockContainerBuilder implements ContainerBuilder for testing
type mockContainerBuilder struct {
	buildFunc           func(ctx context.Context, cfg Config) (*BuildResult, error)
	pushFunc            func(ctx context.Context, imageRef, registry string) (string, error)
	pushDigestFunc      func(ctx context.Context, imageRef, registry string) (string, error)
	tagFunc             func(ctx context.Context, imageRef, newTag string) error
	removeFunc          func(ctx context.Context, imageRef string) error
	closeFunc           func() error
	setCacheOptionsFunc func(ctx context.Context, cacheFrom, cacheTo []string)
	createManifestFunc  func(manifestName string, entries []manifests.ManifestEntry) error

	manifestMu sync.Mutex
	manifests  map[string][]manifests.ManifestEntry
}

func (m *mockContainerBuilder) Build(ctx context.Context, cfg Config) (*BuildResult, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, cfg)
	}
	return &BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "amd64",
		Platform:     "linux/amd64",
		Digest:       "sha256:1234567890abcdef",
		Duration:     "1s",
	}, nil
}

func (m *mockContainerBuilder) Push(ctx context.Context, imageRef, registry string) (string, error) {
	if m.pushFunc != nil {
		return m.pushFunc(ctx, imageRef, registry)
	}
	return "sha256:1234567890abcdef", nil
}

func (m *mockContainerBuilder) PushDigest(ctx context.Context, imageRef, registry string) (string, error) {
	if m.pushDigestFunc != nil {
		return m.pushDigestFunc(ctx, imageRef, registry)
	}
	return "sha256:1234567890abcdef", nil
}

func (m *mockContainerBuilder) Tag(ctx context.Context, imageRef, newTag string) error {
	if m.tagFunc != nil {
		return m.tagFunc(ctx, imageRef, newTag)
	}
	return nil
}

func (m *mockContainerBuilder) Remove(ctx context.Context, imageRef string) error {
	if m.removeFunc != nil {
		return m.removeFunc(ctx, imageRef)
	}
	return nil
}

func (m *mockContainerBuilder) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockContainerBuilder) CreateAndPushManifest(_ context.Context, manifestName string, entries []manifests.ManifestEntry) error {
	if m.createManifestFunc != nil {
		return m.createManifestFunc(manifestName, entries)
	}

	m.manifestMu.Lock()
	defer m.manifestMu.Unlock()
	if m.manifests == nil {
		m.manifests = map[string][]manifests.ManifestEntry{}
	}
	m.manifests[manifestName] = entries

	return nil
}

// manifestNames returns the manifest list names published so far, sorted.
func (m *mockContainerBuilder) manifestNames() []string {
	m.manifestMu.Lock()
	defer m.manifestMu.Unlock()

	names := make([]string, 0, len(m.manifests))
	for name := range m.manifests {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (m *mockContainerBuilder) SetCacheOptions(ctx context.Context, cacheFrom, cacheTo []string) {
	if m.setCacheOptionsFunc != nil {
		m.setCacheOptionsFunc(ctx, cacheFrom, cacheTo)
	}
}

func (m *mockContainerBuilder) SupportsMultiArch() bool {
	return true
}

func TestNewBuildService(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{}, nil
	}
	service := NewBuildService(cfg, buildKitCreator)

	if service == nil {
		t.Fatal("NewBuildService() returned nil")
	}

	if service.globalConfig != cfg {
		t.Error("NewBuildService() config not set correctly")
	}
}

func TestDetermineTargetType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config *Config
		opts   BuildOptions
		want   string
	}{
		{
			name:   "CLI override takes precedence",
			config: &Config{},
			opts: BuildOptions{
				TargetType: "ami",
			},
			want: "ami",
		},
		{
			name: "use config target type",
			config: &Config{
				Targets: []Target{
					{Type: "container"},
				},
			},
			opts: BuildOptions{},
			want: "container",
		},
		{
			name:   "default to container",
			config: &Config{},
			opts:   BuildOptions{},
			want:   "container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineTargetType(tt.config, tt.opts)
			if got != tt.want {
				t.Errorf("DetermineTargetType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateManifestEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		results []BuildResult
		wantLen int
		wantErr bool
	}{
		{
			name: "valid results",
			results: []BuildResult{
				{
					ImageRef:     "test-image:amd64",
					Architecture: "amd64",
					Platform:     "linux/amd64",
					Digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				},
				{
					ImageRef:     "test-image:arm64",
					Architecture: "arm64",
					Platform:     "linux/arm64",
					Digest:       "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
				},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "result with invalid digest",
			results: []BuildResult{
				{
					ImageRef:     "test-image:amd64",
					Architecture: "amd64",
					Platform:     "linux/amd64",
					Digest:       "invalid-digest",
				},
			},
			wantLen: 0,
			wantErr: true, // An architecture that cannot be described fails the set
		},
		{
			name:    "empty results",
			results: []BuildResult{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateManifestEntries(context.Background(), tt.results)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateManifestEntries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("CreateManifestEntries() got %d entries, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestCreateManifestEntries_PlatformParsing(t *testing.T) {
	t.Parallel()
	results := []BuildResult{
		{
			ImageRef:     "test-image:arm64v8",
			Architecture: "arm64",
			Platform:     "linux/arm64/v8",
			Digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
	}

	got, err := CreateManifestEntries(context.Background(), results)
	if err != nil {
		t.Errorf("CreateManifestEntries() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("CreateManifestEntries() got %d entries, want 1", len(got))
	}

	entry := got[0]
	if entry.OS != "linux" {
		t.Errorf("CreateManifestEntries() OS = %s, want linux", entry.OS)
	}
	if entry.Architecture != "arm64" {
		t.Errorf("CreateManifestEntries() Architecture = %s, want arm64", entry.Architecture)
	}
	if entry.Variant != "v8" {
		t.Errorf("CreateManifestEntries() Variant = %s, want v8", entry.Variant)
	}
}

func TestBuildService_ExecuteContainerBuild_SingleArch(t *testing.T) {
	cfg := &config.Config{}

	buildCalled := false
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
				buildCalled = true
				return &BuildResult{
					ImageRef:     "test-image:latest",
					Architecture: "amd64",
					Platform:     "linux/amd64",
					Duration:     "1s",
				}, nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	buildOpts := BuildOptions{}

	ctx := context.Background()
	results, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err != nil {
		t.Errorf("ExecuteContainerBuild() error = %v", err)
	}

	if !buildCalled {
		t.Error("ExecuteContainerBuild() did not call Build")
	}

	if len(results) != 1 {
		t.Errorf("ExecuteContainerBuild() got %d results, want 1", len(results))
	}
}

func TestBuildService_ExecuteContainerBuild_BuildError(t *testing.T) {
	cfg := &config.Config{}

	expectedErr := fmt.Errorf("build failed")
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
				return nil, expectedErr
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	buildOpts := BuildOptions{}

	ctx := context.Background()
	_, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err == nil {
		t.Error("ExecuteContainerBuild() expected error, got nil")
	}
}

func TestBuildService_ExecuteContainerBuild_BuilderCreationError(t *testing.T) {
	cfg := &config.Config{}

	expectedErr := fmt.Errorf("builder creation failed")
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return nil, expectedErr
	}

	service := NewBuildService(cfg, buildKitCreator)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	buildOpts := BuildOptions{}

	ctx := context.Background()
	_, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err == nil {
		t.Error("ExecuteContainerBuild() expected error, got nil")
	}
}

func TestManifestEntry_DigestParsing(t *testing.T) {
	t.Parallel()
	result := BuildResult{
		ImageRef:     "test-image:amd64",
		Architecture: "amd64",
		Platform:     "linux/amd64",
		Digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	entries, err := CreateManifestEntries(context.Background(), []BuildResult{result})
	if err != nil {
		t.Errorf("CreateManifestEntries() error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("CreateManifestEntries() got %d entries, want 1", len(entries))
	}

	entry := entries[0]
	expectedDigest, _ := digest.Parse("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	if entry.Digest.String() != expectedDigest.String() {
		t.Errorf("ManifestEntry Digest = %v, want %v", entry.Digest, expectedDigest)
	}
}

func TestBuildService_SaveDigests(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	service := NewBuildService(cfg, nil)

	results := []BuildResult{
		{
			ImageRef:     "test-image:amd64",
			Architecture: "amd64",
			Digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// This should not error even if it can't write the files
	service.saveDigests(ctx, "test-image", results, tmpDir)

	// Note: Actual file writing is tested in the manifests package
}

func TestBuildService_PushSingleArch_WithoutDigest(t *testing.T) {
	cfg := &config.Config{}

	pushCalled := false
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
				pushCalled = true
				if registry != "ghcr.io/myorg" {
					return "", fmt.Errorf("unexpected registry: %s", registry)
				}
				return "sha256:1234567890abcdef", nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	result := BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "amd64",
	}

	buildOpts := BuildOptions{
		Registry: "ghcr.io/myorg",
	}

	ctx := context.Background()
	err := service.pushSingleArch(ctx, buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			pushCalled = true
			return "sha256:1234567890abcdef", nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}

	if !pushCalled {
		t.Error("pushSingleArch() did not call Push")
	}
}

func TestBuildService_PushSingleArch_DigestOnly(t *testing.T) {
	cfg := &config.Config{}
	ctx := context.Background()
	pushDigestCalled := false

	service := NewBuildService(cfg, nil)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	result := BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "amd64",
	}

	buildOpts := BuildOptions{
		Registry:   "ghcr.io/myorg",
		PushDigest: true,
	}

	err := service.pushSingleArch(ctx, buildConfig, result, &mockContainerBuilder{
		pushDigestFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			pushDigestCalled = true
			return "sha256:1234567890abcdef", nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}

	if !pushDigestCalled {
		t.Error("pushSingleArch() did not call PushDigest")
	}
}

func TestBuildService_PushSingleArch_UsesResultArchitecture(t *testing.T) {
	cfg := &config.Config{}
	ctx := context.Background()

	// Track what architecture was used when saving digest
	var capturedArch string

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
				return "sha256:testdigest123", nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	// Create a config with Architectures set to something different from BuildResult
	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"wrong-arch"}, // This should NOT be used
	}

	// BuildResult has the correct architecture
	result := BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "arm64", // This SHOULD be used
		Digest:       "sha256:abc123",
	}

	digestDir := t.TempDir()
	buildOpts := BuildOptions{
		Registry:    "ghcr.io/test",
		SaveDigests: true,
		DigestDir:   digestDir,
	}

	err := service.pushSingleArch(ctx, buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			return "sha256:testdigest123", nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}

	// Verify the digest file was created with the correct architecture in the filename
	// The file should be named: digest-test-arm64.txt (using result.Architecture, not config.Architectures)
	expectedFile := fmt.Sprintf("%s/digest-test-arm64.txt", digestDir)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected digest file not found: %s", expectedFile)
	}

	// Verify the WRONG file was NOT created
	wrongFile := fmt.Sprintf("%s/digest-test-wrong-arch.txt", digestDir)
	if _, err := os.Stat(wrongFile); err == nil {
		t.Errorf("Wrong digest file should not exist: %s", wrongFile)
	}

	_ = capturedArch
}

func TestBuildService_ApplyOverridesBeforeBuild(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{},
		Registry:      "",
	}

	buildOpts := BuildOptions{
		Architectures: []string{"amd64", "arm64"},
		Registry:      "ghcr.io/myorg",
	}

	ctx := context.Background()
	_, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err != nil {
		t.Errorf("ExecuteContainerBuild() error = %v", err)
	}

	// Config should have been modified by ApplyOverrides
	// Note: The config is modified in place, but we passed a copy,
	// so we can't verify the changes here. This is more of an integration test.
}

// mockAMIBuilder implements AMIBuilder for testing
type mockAMIBuilder struct {
	buildFunc      func(ctx context.Context, cfg Config) (*BuildResult, error)
	shareFunc      func(ctx context.Context, amiID string, accountIDs []string) error
	copyFunc       func(ctx context.Context, amiID, sourceRegion, destRegion string) (string, error)
	deregisterFunc func(ctx context.Context, amiID, region string) error
	closeFunc      func() error
}

func (m *mockAMIBuilder) Build(ctx context.Context, cfg Config) (*BuildResult, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, cfg)
	}
	return &BuildResult{
		AMIID:    "ami-12345678",
		Region:   "us-east-1",
		Duration: "5m",
	}, nil
}

func (m *mockAMIBuilder) Share(ctx context.Context, amiID string, accountIDs []string) error {
	if m.shareFunc != nil {
		return m.shareFunc(ctx, amiID, accountIDs)
	}
	return nil
}

func (m *mockAMIBuilder) Copy(ctx context.Context, amiID, sourceRegion, destRegion string) (string, error) {
	if m.copyFunc != nil {
		return m.copyFunc(ctx, amiID, sourceRegion, destRegion)
	}
	return "ami-copy-12345678", nil
}

func (m *mockAMIBuilder) Deregister(ctx context.Context, amiID, region string) error {
	if m.deregisterFunc != nil {
		return m.deregisterFunc(ctx, amiID, region)
	}
	return nil
}

func (m *mockAMIBuilder) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestExecuteAMIBuild_NoAMITarget(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "container"}},
	}

	buildOpts := BuildOptions{Region: "us-east-1"}
	amiBuilder := &mockAMIBuilder{}

	_, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err == nil {
		t.Error("ExecuteAMIBuild() expected error for missing AMI target, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no AMI target found") {
		t.Errorf("ExecuteAMIBuild() error = %v, want 'no AMI target found'", err)
	}
}

func TestExecuteAMIBuild_MissingRegion(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "ami"}},
	}

	buildOpts := BuildOptions{}
	amiBuilder := &mockAMIBuilder{}

	_, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err == nil {
		t.Error("ExecuteAMIBuild() expected error for missing region, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "AWS region must be specified") {
		t.Errorf("ExecuteAMIBuild() error = %v, want 'AWS region must be specified'", err)
	}
}

func TestExecuteAMIBuild_RegionFromOpts(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "ami"}},
	}

	buildOpts := BuildOptions{Region: "us-west-2"}
	amiBuilder := &mockAMIBuilder{}

	result, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err != nil {
		t.Errorf("ExecuteAMIBuild() error = %v", err)
	}
	if result == nil {
		t.Fatal("ExecuteAMIBuild() returned nil result")
	}
	if result.AMIID != "ami-12345678" {
		t.Errorf("ExecuteAMIBuild() AMIID = %s, want ami-12345678", result.AMIID)
	}
}

func TestExecuteAMIBuild_RegionFromTargetConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "ami", Region: "eu-west-1"}},
	}

	buildOpts := BuildOptions{}
	amiBuilder := &mockAMIBuilder{}

	result, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err != nil {
		t.Errorf("ExecuteAMIBuild() error = %v", err)
	}
	if result == nil {
		t.Fatal("ExecuteAMIBuild() returned nil result")
	}
}

func TestExecuteAMIBuild_RegionFromGlobalConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AWS: config.AWSConfig{Region: "ap-southeast-1"},
	}
	service := NewBuildService(cfg, nil)

	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "ami"}},
	}

	buildOpts := BuildOptions{}
	amiBuilder := &mockAMIBuilder{}

	result, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err != nil {
		t.Errorf("ExecuteAMIBuild() error = %v", err)
	}
	if result == nil {
		t.Fatal("ExecuteAMIBuild() returned nil result")
	}
}

func TestExecuteAMIBuild_BuildError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "ami"}},
	}

	buildOpts := BuildOptions{Region: "us-east-1"}
	amiBuilder := &mockAMIBuilder{
		buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
			return nil, fmt.Errorf("AMI pipeline failed")
		},
	}

	_, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err == nil {
		t.Error("ExecuteAMIBuild() expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "AMI build failed") {
		t.Errorf("ExecuteAMIBuild() error = %v, want 'AMI build failed'", err)
	}
}

func TestPush_EmptyRegistry(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	results := []BuildResult{
		{ImageRef: "test:latest", Architecture: "amd64"},
	}

	buildOpts := BuildOptions{Registry: ""}

	err := service.Push(context.Background(), Config{Name: "test"}, results, buildOpts)
	if err == nil {
		t.Error("Push() expected error for empty registry, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "registry must be specified") {
		t.Errorf("Push() error = %v, want 'registry must be specified'", err)
	}
}

func TestPush_BuilderCreationError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return nil, fmt.Errorf("cannot create builder")
	}

	service := NewBuildService(cfg, buildKitCreator)

	results := []BuildResult{
		{ImageRef: "test:latest", Architecture: "amd64"},
	}

	buildOpts := BuildOptions{Registry: "ghcr.io/myorg"}

	err := service.Push(context.Background(), Config{Name: "test"}, results, buildOpts)
	if err == nil {
		t.Error("Push() expected error for builder creation failure, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to create builder for push") {
		t.Errorf("Push() error = %v, want 'failed to create builder for push'", err)
	}
}

func TestPush_SingleResult(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	pushCalled := false
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
				pushCalled = true
				return "sha256:" + strings.Repeat("c", 64), nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	results := []BuildResult{
		{ImageRef: "test:latest", Architecture: "amd64"},
	}

	buildOpts := BuildOptions{Registry: "ghcr.io/myorg"}

	err := service.Push(context.Background(), Config{Name: "test"}, results, buildOpts)
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}
	if !pushCalled {
		t.Error("Push() did not call pushSingleArch path")
	}
}

func TestPush_MultipleResults(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	var pushCount atomic.Int32
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
				pushCount.Add(1)
				return "sha256:" + strings.Repeat("c", 64), nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	results := []BuildResult{
		{ImageRef: "test:amd64", Architecture: "amd64", Platform: "linux/amd64"},
		{ImageRef: "test:arm64", Architecture: "arm64", Platform: "linux/arm64"},
	}

	buildOpts := BuildOptions{Registry: "ghcr.io/myorg"}

	err := service.Push(context.Background(), Config{Name: "test"}, results, buildOpts)
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}
}

func TestSaveDigests_EmptyDigests(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	results := []BuildResult{
		{
			ImageRef:     "test-image:amd64",
			Architecture: "amd64",
			Digest:       "", // Empty digest should be skipped
		},
		{
			ImageRef:     "test-image:arm64",
			Architecture: "arm64",
			Digest:       "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
		},
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	service.saveDigests(ctx, "test-image", results, tmpDir)

	// The empty digest result should be skipped; only arm64 should be saved
	emptyDigestFile := fmt.Sprintf("%s/digest-test-image-amd64.txt", tmpDir)
	if _, err := os.Stat(emptyDigestFile); err == nil {
		t.Error("saveDigests() should not create file for empty digest")
	}

	arm64File := fmt.Sprintf("%s/digest-test-image-arm64.txt", tmpDir)
	if _, err := os.Stat(arm64File); os.IsNotExist(err) {
		t.Errorf("saveDigests() should create file for non-empty digest: %s", arm64File)
	}
}

func TestSaveDigests_AllEmpty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	results := []BuildResult{
		{ImageRef: "test:amd64", Architecture: "amd64", Digest: ""},
		{ImageRef: "test:arm64", Architecture: "arm64", Digest: ""},
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Should not panic even with all empty digests
	service.saveDigests(ctx, "test-image", results, tmpDir)
}

func TestExecuteContainerBuild_MultiArch(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	var buildCount atomic.Int32
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
				buildCount.Add(1)
				return &BuildResult{
					ImageRef:     fmt.Sprintf("test:%s", cfg.Base.Platform),
					Architecture: cfg.Architectures[0],
					Platform:     cfg.Base.Platform,
					Duration:     "1s",
				}, nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64", "arm64"},
	}

	buildOpts := BuildOptions{}

	ctx := context.Background()
	results, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err != nil {
		t.Errorf("ExecuteContainerBuild() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("ExecuteContainerBuild() got %d results, want 2", len(results))
	}
}

func TestPushSingleArch_PushError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	result := BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "amd64",
	}

	buildOpts := BuildOptions{
		Registry: "ghcr.io/myorg",
	}

	err := service.pushSingleArch(context.Background(), buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			return "", fmt.Errorf("push authentication failed")
		},
	}, buildOpts)

	if err == nil {
		t.Error("pushSingleArch() expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to push image") {
		t.Errorf("pushSingleArch() error = %v, want 'failed to push image'", err)
	}
}

func TestPushSingleArch_FallbackArchFromConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	// Result has no architecture set - should fall back to config.Architectures[0]
	result := BuildResult{
		ImageRef: "test-image:latest",
	}

	digestDir := t.TempDir()
	buildOpts := BuildOptions{
		Registry:    "ghcr.io/myorg",
		SaveDigests: true,
		DigestDir:   digestDir,
	}

	err := service.pushSingleArch(context.Background(), buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			return "sha256:testdigest123456789012345678901234567890123456789012345678901234", nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}

	// Verify fallback to config architecture
	expectedFile := fmt.Sprintf("%s/digest-test-amd64.txt", digestDir)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected digest file using fallback arch not found: %s", expectedFile)
	}
}

func TestPushSingleArch_UnknownArchFallback(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{}, // No architectures in config either
	}

	// No architecture in result or config
	result := BuildResult{
		ImageRef: "test-image:latest",
	}

	digestDir := t.TempDir()
	buildOpts := BuildOptions{
		Registry:    "ghcr.io/myorg",
		SaveDigests: true,
		DigestDir:   digestDir,
	}

	err := service.pushSingleArch(context.Background(), buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			return "sha256:testdigest123456789012345678901234567890123456789012345678901234", nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}

	// Should use "unknown" as fallback architecture
	expectedFile := fmt.Sprintf("%s/digest-test-unknown.txt", digestDir)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Expected digest file using 'unknown' arch not found: %s", expectedFile)
	}
}

func TestPushSingleArch_SaveDigestDisabled(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	service := NewBuildService(cfg, nil)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	result := BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "amd64",
		Digest:       "sha256:testdigest",
	}

	digestDir := t.TempDir()
	buildOpts := BuildOptions{
		Registry:    "ghcr.io/myorg",
		SaveDigests: false, // digest saving disabled
		DigestDir:   digestDir,
	}

	err := service.pushSingleArch(context.Background(), buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
			return "sha256:" + strings.Repeat("c", 64), nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}
}

func TestCreateManifestEntries_EmptyDigest(t *testing.T) {
	t.Parallel()
	results := []BuildResult{
		{
			ImageRef:     "test-image:amd64",
			Architecture: "amd64",
			Platform:     "linux/amd64",
			Digest:       "", // Empty digest should result in zero-value digest
		},
	}

	got, err := CreateManifestEntries(context.Background(), results)
	if err != nil {
		t.Errorf("CreateManifestEntries() error = %v", err)
	}

	if len(got) != 1 {
		t.Errorf("CreateManifestEntries() got %d entries, want 1 (empty digest produces zero-value entry)", len(got))
	}
}

func TestExecuteAMIBuild_RegionPrecedence(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AWS: config.AWSConfig{Region: "global-region"},
	}
	service := NewBuildService(cfg, nil)

	// CLI region should take precedence over target and global
	buildConfig := Config{
		Name:    "test",
		Targets: []Target{{Type: "ami", Region: "target-region"}},
	}

	buildOpts := BuildOptions{Region: "cli-region"}

	var capturedRegion string
	amiBuilder := &mockAMIBuilder{
		buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
			return &BuildResult{
				AMIID:    "ami-12345678",
				Region:   "cli-region",
				Duration: "5m",
			}, nil
		},
	}

	result, err := service.ExecuteAMIBuild(context.Background(), buildConfig, buildOpts, amiBuilder)
	if err != nil {
		t.Errorf("ExecuteAMIBuild() error = %v", err)
	}
	if result == nil {
		t.Fatal("ExecuteAMIBuild() returned nil result")
	}

	_ = capturedRegion
}

func TestPushMultiArch_SaveDigests(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
				return "sha256:" + strings.Repeat("c", 64), nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator)

	results := []BuildResult{
		{ImageRef: "test:amd64", Architecture: "amd64", Platform: "linux/amd64", Digest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
		{ImageRef: "test:arm64", Architecture: "arm64", Platform: "linux/arm64", Digest: "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"},
	}

	digestDir := t.TempDir()
	buildOpts := BuildOptions{
		Registry:    "ghcr.io/myorg",
		SaveDigests: true,
		DigestDir:   digestDir,
	}

	err := service.Push(context.Background(), Config{Name: "test"}, results, buildOpts)
	if err != nil {
		t.Errorf("Push() error = %v", err)
	}

	// Verify digest files were created for both architectures
	for _, arch := range []string{"amd64", "arm64"} {
		expectedFile := fmt.Sprintf("%s/digest-test-%s.txt", digestDir, arch)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Expected digest file not found: %s", expectedFile)
		}
	}
}

// mockAzureImageBuilder implements AzureImageBuilder for testing the
// post-build sharing flow inside ExecuteAzureBuild.
type mockAzureImageBuilder struct {
	buildFunc func(ctx context.Context, cfg Config) (*BuildResult, error)

	shareCalled    bool
	shareVersionID string
	sharePrincipal []string
	shareErr       error

	replicateCalled bool
	deleteCalled    bool
}

func (m *mockAzureImageBuilder) Build(ctx context.Context, cfg Config) (*BuildResult, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, cfg)
	}
	return &BuildResult{GalleryImageVersionID: "/v/123", Location: "eastus"}, nil
}

func (m *mockAzureImageBuilder) Replicate(_ context.Context, _ string, _ []string) error {
	m.replicateCalled = true
	return nil
}

func (m *mockAzureImageBuilder) Share(_ context.Context, versionID string, principals []string) error {
	m.shareCalled = true
	m.shareVersionID = versionID
	m.sharePrincipal = principals
	return m.shareErr
}

func (m *mockAzureImageBuilder) Delete(_ context.Context, _ string) error {
	m.deleteCalled = true
	return nil
}

func (m *mockAzureImageBuilder) Close() error { return nil }

func azureBuildConfig(shareWith []string) Config {
	return Config{
		Name: "img",
		Targets: []Target{
			{
				Type:                   "azure",
				SubscriptionID:         "sub",
				ResourceGroup:          "rg",
				Location:               "eastus",
				Gallery:                "g",
				GalleryImageDefinition: "def",
				IdentityID:             "/uami",
				ShareWith:              shareWith,
			},
		},
	}
}

func TestExecuteAzureBuild_NoShareWith(t *testing.T) {
	mb := &mockAzureImageBuilder{}
	svc := NewBuildService(nil, nil)

	res, err := svc.ExecuteAzureBuild(context.Background(), azureBuildConfig(nil), BuildOptions{}, mb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected build result, got nil")
	}
	if mb.shareCalled {
		t.Errorf("Share should not have been called when share_with is empty")
	}
}

func TestExecuteAzureBuild_SharesAfterSuccess(t *testing.T) {
	mb := &mockAzureImageBuilder{}
	svc := NewBuildService(nil, nil)

	cfg := azureBuildConfig([]string{"principal-1", "principal-2"})
	_, err := svc.ExecuteAzureBuild(context.Background(), cfg, BuildOptions{}, mb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mb.shareCalled {
		t.Fatal("Share should have been called")
	}
	if mb.shareVersionID != "/v/123" {
		t.Errorf("expected versionID /v/123, got %q", mb.shareVersionID)
	}
	if len(mb.sharePrincipal) != 2 || mb.sharePrincipal[0] != "principal-1" {
		t.Errorf("unexpected principals: %v", mb.sharePrincipal)
	}
}

func TestExecuteAzureBuild_ShareErrorPropagates(t *testing.T) {
	mb := &mockAzureImageBuilder{shareErr: fmt.Errorf("rbac denied")}
	svc := NewBuildService(nil, nil)

	cfg := azureBuildConfig([]string{"p"})
	_, err := svc.ExecuteAzureBuild(context.Background(), cfg, BuildOptions{}, mb)
	if err == nil {
		t.Fatal("expected error from Share to propagate")
	}
	if !strings.Contains(err.Error(), "rbac denied") {
		t.Errorf("error did not contain expected message: %v", err)
	}
}

func TestExecuteAzureBuild_BuildFailureSkipsShare(t *testing.T) {
	mb := &mockAzureImageBuilder{
		buildFunc: func(_ context.Context, _ Config) (*BuildResult, error) {
			return nil, fmt.Errorf("build broke")
		},
	}
	svc := NewBuildService(nil, nil)

	_, err := svc.ExecuteAzureBuild(context.Background(), azureBuildConfig([]string{"p"}), BuildOptions{}, mb)
	if err == nil {
		t.Fatal("expected build error to propagate")
	}
	if mb.shareCalled {
		t.Error("Share must not be called when Build fails")
	}
}

func TestExecuteAzureBuild_NoAzureTarget(t *testing.T) {
	mb := &mockAzureImageBuilder{}
	svc := NewBuildService(nil, nil)

	cfg := Config{
		Name:    "img",
		Targets: []Target{{Type: "ami", Region: "us-east-1"}},
	}
	_, err := svc.ExecuteAzureBuild(context.Background(), cfg, BuildOptions{}, mb)
	if err == nil || !strings.Contains(err.Error(), "no azure target found") {
		t.Fatalf("expected 'no azure target found' error, got %v", err)
	}
}

func TestExecuteAzureBuild_MissingLocationErrors(t *testing.T) {
	mb := &mockAzureImageBuilder{}
	svc := NewBuildService(nil, nil)

	cfg := Config{
		Name: "img",
		Targets: []Target{
			{
				Type:                   "azure",
				ResourceGroup:          "rg",
				Gallery:                "g",
				GalleryImageDefinition: "def",
				IdentityID:             "/uami",
			},
		},
	}
	_, err := svc.ExecuteAzureBuild(context.Background(), cfg, BuildOptions{}, mb)
	if err == nil || !strings.Contains(err.Error(), "azure location must be specified") {
		t.Fatalf("expected location error, got %v", err)
	}
}

func TestApplyAzureGlobalDefaults(t *testing.T) {
	tests := []struct {
		name       string
		globalCfg  *config.Config
		target     Target
		wantTarget Target
	}{
		{
			name:      "nil global config leaves target untouched",
			globalCfg: nil,
			target: Target{
				Type:     "azure",
				Location: "eastus",
			},
			wantTarget: Target{
				Type:     "azure",
				Location: "eastus",
			},
		},
		{
			name: "fills empty fields from global config",
			globalCfg: &config.Config{
				Azure: config.AzureConfig{
					SubscriptionID: "sub-global",
					ResourceGroup:  "rg-global",
					Location:       "westus",
					IdentityID:     "/uami-global",
					Image:          config.AzureImageConfig{VMSize: "Standard_D2s_v3"},
				},
			},
			target: Target{Type: "azure"},
			wantTarget: Target{
				Type:           "azure",
				SubscriptionID: "sub-global",
				ResourceGroup:  "rg-global",
				Location:       "westus",
				IdentityID:     "/uami-global",
				VMSize:         "Standard_D2s_v3",
			},
		},
		{
			name: "target values take precedence over global defaults",
			globalCfg: &config.Config{
				Azure: config.AzureConfig{
					SubscriptionID: "sub-global",
					ResourceGroup:  "rg-global",
					Location:       "westus",
					IdentityID:     "/uami-global",
					Image:          config.AzureImageConfig{VMSize: "Standard_D2s_v3"},
				},
			},
			target: Target{
				Type:           "azure",
				SubscriptionID: "sub-target",
				ResourceGroup:  "rg-target",
				Location:       "eastus",
				IdentityID:     "/uami-target",
				VMSize:         "Standard_E4s_v3",
			},
			wantTarget: Target{
				Type:           "azure",
				SubscriptionID: "sub-target",
				ResourceGroup:  "rg-target",
				Location:       "eastus",
				IdentityID:     "/uami-target",
				VMSize:         "Standard_E4s_v3",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewBuildService(tc.globalCfg, nil)
			got := tc.target
			svc.applyAzureGlobalDefaults(&got)
			if !reflect.DeepEqual(got, tc.wantTarget) {
				t.Errorf("applyAzureGlobalDefaults() = %+v, want %+v", got, tc.wantTarget)
			}
		})
	}
}

func TestFindAzureTarget(t *testing.T) {
	tests := []struct {
		name    string
		targets []Target
		wantNil bool
	}{
		{
			name:    "empty targets returns nil",
			targets: nil,
			wantNil: true,
		},
		{
			name: "no azure target returns nil",
			targets: []Target{
				{Type: "ami", Region: "us-east-1"},
				{Type: "container"},
			},
			wantNil: true,
		},
		{
			name: "returns first azure target",
			targets: []Target{
				{Type: "container"},
				{Type: "azure", Location: "eastus"},
				{Type: "azure", Location: "westus"},
			},
			wantNil: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findAzureTarget(tc.targets)
			if tc.wantNil && got != nil {
				t.Errorf("expected nil, got %+v", got)
			}
			if !tc.wantNil {
				if got == nil {
					t.Fatal("expected non-nil target")
				}
				if got.Location != "eastus" {
					t.Errorf("expected first azure target location eastus, got %q", got.Location)
				}
			}
		})
	}
}

func TestExecuteAzureBuild_AppliesGlobalDefaults(t *testing.T) {
	mb := &mockAzureImageBuilder{}
	globalCfg := &config.Config{
		Azure: config.AzureConfig{
			Location: "westus",
		},
	}
	svc := NewBuildService(globalCfg, nil)

	cfg := Config{
		Name: "img",
		Targets: []Target{
			{
				Type:                   "azure",
				ResourceGroup:          "rg",
				Gallery:                "g",
				GalleryImageDefinition: "def",
				IdentityID:             "/uami",
			},
		},
	}
	res, err := svc.ExecuteAzureBuild(context.Background(), cfg, BuildOptions{}, mb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected build result")
	}
}

// mockProxmoxImageBuilder implements ProxmoxImageBuilder for testing the
// ExecuteProxmoxBuild orchestration. Mirrors mockAzureImageBuilder.
type mockProxmoxImageBuilder struct {
	buildFunc func(ctx context.Context, cfg Config) (*BuildResult, error)

	shareCalled  bool
	deleteCalled bool
}

func (m *mockProxmoxImageBuilder) Build(ctx context.Context, cfg Config) (*BuildResult, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, cfg)
	}
	return &BuildResult{TemplateVMID: 9100, TemplateName: "img-test", Node: "pve1"}, nil
}

func (m *mockProxmoxImageBuilder) Share(_ context.Context, _ int, _ []string) error {
	m.shareCalled = true
	return nil
}

func (m *mockProxmoxImageBuilder) Delete(_ context.Context, _ int) error {
	m.deleteCalled = true
	return nil
}

func (m *mockProxmoxImageBuilder) Close() error { return nil }

func proxmoxBuildConfig(node string) Config {
	return Config{
		Name: "img",
		Targets: []Target{
			{
				Type:           "proxmox",
				Node:           node,
				SourceTemplate: 9000,
				TemplateName:   "img",
			},
		},
	}
}

func TestExecuteProxmoxBuild_HappyPath(t *testing.T) {
	mb := &mockProxmoxImageBuilder{}
	svc := NewBuildService(nil, nil)

	res, err := svc.ExecuteProxmoxBuild(context.Background(), proxmoxBuildConfig("pve1"), BuildOptions{}, mb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.TemplateVMID != 9100 {
		t.Fatalf("expected TemplateVMID 9100, got %+v", res)
	}
}

func TestExecuteProxmoxBuild_NoProxmoxTarget(t *testing.T) {
	mb := &mockProxmoxImageBuilder{}
	svc := NewBuildService(nil, nil)

	_, err := svc.ExecuteProxmoxBuild(context.Background(), Config{Targets: []Target{{Type: "container"}}}, BuildOptions{}, mb)
	if err == nil || !strings.Contains(err.Error(), "no proxmox target") {
		t.Fatalf("expected no-target error, got %v", err)
	}
}

func TestExecuteProxmoxBuild_MissingNode(t *testing.T) {
	mb := &mockProxmoxImageBuilder{}
	svc := NewBuildService(nil, nil)

	_, err := svc.ExecuteProxmoxBuild(context.Background(), proxmoxBuildConfig(""), BuildOptions{}, mb)
	if err == nil || !strings.Contains(err.Error(), "node must be specified") {
		t.Fatalf("expected node-required error, got %v", err)
	}
}

func TestExecuteProxmoxBuild_BuildErrorPropagates(t *testing.T) {
	mb := &mockProxmoxImageBuilder{
		buildFunc: func(_ context.Context, _ Config) (*BuildResult, error) {
			return nil, fmt.Errorf("clone failed")
		},
	}
	svc := NewBuildService(nil, nil)

	_, err := svc.ExecuteProxmoxBuild(context.Background(), proxmoxBuildConfig("pve1"), BuildOptions{}, mb)
	if err == nil || !strings.Contains(err.Error(), "clone failed") {
		t.Fatalf("expected build error to propagate, got %v", err)
	}
}

func TestExecuteProxmoxBuild_GlobalDefaultsAppliedWhenTargetEmpty(t *testing.T) {
	mb := &mockProxmoxImageBuilder{}
	svc := NewBuildService(&config.Config{
		Proxmox: config.ProxmoxConfig{
			Node:    "pve-global",
			Storage: "local-zfs",
			Pool:    "deploy",
		},
	}, nil)

	// Target with empty Node/Storage/Pool should pick up global defaults.
	cfg := Config{
		Name: "img",
		Targets: []Target{{
			Type:           "proxmox",
			SourceTemplate: 9000,
			TemplateName:   "img",
		}},
	}
	_, err := svc.ExecuteProxmoxBuild(context.Background(), cfg, BuildOptions{}, mb)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.Targets[0].Node != "pve-global" {
		t.Fatalf("expected target node defaulted from global, got %q", cfg.Targets[0].Node)
	}
	if cfg.Targets[0].Storage != "local-zfs" {
		t.Fatalf("expected storage defaulted from global, got %q", cfg.Targets[0].Storage)
	}
	if cfg.Targets[0].Pool != "deploy" {
		t.Fatalf("expected pool defaulted from global, got %q", cfg.Targets[0].Pool)
	}
}

func TestFindProxmoxTarget(t *testing.T) {
	cfg := []Target{{Type: "container"}, {Type: "proxmox", Node: "pve1"}}
	if got := findProxmoxTarget(cfg); got == nil || got.Node != "pve1" {
		t.Fatalf("expected to find proxmox target, got %+v", got)
	}
	if got := findProxmoxTarget([]Target{{Type: "ami"}}); got != nil {
		t.Fatalf("expected nil when no proxmox target, got %+v", got)
	}
}

func TestPushAdditionalTags(t *testing.T) {
	tests := []struct {
		name       string
		refs       []string
		pushDigest bool
		wantPushed []string
		wantErr    bool
	}{
		{
			name: "nothing to push",
		},
		{
			name:       "every additional reference is pushed",
			refs:       []string{"ghcr.io/cowdogmoo/mealie:stable", "ghcr.io/cowdogmoo/mealie:edge"},
			wantPushed: []string{"ghcr.io/cowdogmoo/mealie:stable", "ghcr.io/cowdogmoo/mealie:edge"},
		},
		{
			name:       "a digest push publishes no tags",
			refs:       []string{"ghcr.io/cowdogmoo/mealie:stable"},
			pushDigest: true,
		},
		{
			name:    "a failed tag push is reported",
			refs:    []string{"ghcr.io/cowdogmoo/mealie:stable"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pushed []string
			bldr := &mockContainerBuilder{
				pushFunc: func(_ context.Context, imageRef, _ string) (string, error) {
					pushed = append(pushed, imageRef)
					if tt.wantErr {
						return "", fmt.Errorf("registry rejected the push")
					}
					return "", nil
				},
			}

			err := pushAdditionalTags(context.Background(), tt.refs, "ghcr.io/cowdogmoo", tt.pushDigest, bldr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("pushAdditionalTags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(pushed, tt.wantPushed) {
				t.Errorf("pushed = %v, want %v", pushed, tt.wantPushed)
			}
		})
	}
}

// TestPushSingleArchPushesAdditionalTags checks the extra references travel with
// the image through the push path, not just through the builder.
func TestPushSingleArchPushesAdditionalTags(t *testing.T) {
	var pushed []string
	bldr := &mockContainerBuilder{
		pushFunc: func(_ context.Context, imageRef, _ string) (string, error) {
			pushed = append(pushed, imageRef)
			return "sha256:abc", nil
		},
	}

	service := NewBuildService(nil, nil)
	result := BuildResult{
		ImageRef:       "ghcr.io/cowdogmoo/mealie:v3.24.0",
		AdditionalRefs: []string{"ghcr.io/cowdogmoo/mealie:latest"},
		Architecture:   "amd64",
	}

	err := service.pushSingleArch(context.Background(), &Config{Name: "mealie"}, result, bldr, BuildOptions{Registry: "ghcr.io/cowdogmoo"})
	if err != nil {
		t.Fatalf("pushSingleArch() error = %v", err)
	}

	want := []string{"ghcr.io/cowdogmoo/mealie:v3.24.0", "ghcr.io/cowdogmoo/mealie:latest"}
	if !reflect.DeepEqual(pushed, want) {
		t.Errorf("pushed = %v, want %v", pushed, want)
	}
}

// TestPushSingleArchAdditionalTagFailure checks a rejected extra tag fails the
// push rather than being reported as a success on the strength of the first one.
func TestPushSingleArchAdditionalTagFailure(t *testing.T) {
	bldr := &mockContainerBuilder{
		pushFunc: func(_ context.Context, imageRef, _ string) (string, error) {
			if strings.HasSuffix(imageRef, ":latest") {
				return "", fmt.Errorf("registry rejected the push")
			}
			return "sha256:abc", nil
		},
	}

	service := NewBuildService(nil, nil)
	result := BuildResult{
		ImageRef:       "ghcr.io/cowdogmoo/mealie:v3.24.0",
		AdditionalRefs: []string{"ghcr.io/cowdogmoo/mealie:latest"},
	}

	err := service.pushSingleArch(context.Background(), &Config{Name: "mealie"}, result, bldr, BuildOptions{Registry: "ghcr.io/cowdogmoo"})
	if err == nil {
		t.Fatal("expected an error when an additional tag fails to push")
	}
	if !strings.Contains(err.Error(), "ghcr.io/cowdogmoo/mealie:latest") {
		t.Errorf("error = %v, want it to name the tag that failed", err)
	}
}

// recordingBuilder mirrors what the BuildKit builder derives from the config it
// is handed, so the orchestration under test is exercised rather than stubbed.
func recordingBuilder(built *[]string, pushed *[]string, mu *sync.Mutex) *mockContainerBuilder {
	return &mockContainerBuilder{
		buildFunc: func(_ context.Context, cfg Config) (*BuildResult, error) {
			result := &BuildResult{
				ImageRef:       PrimaryImageRef(cfg),
				AdditionalRefs: AdditionalTagRefs(cfg),
				Architecture:   cfg.Version,
				Platform:       cfg.Base.Platform,
				Digest:         "sha256:" + strings.Repeat("a", 64),
			}

			mu.Lock()
			*built = append(*built, result.ImageRef)
			*built = append(*built, result.AdditionalRefs...)
			mu.Unlock()

			return result, nil
		},
		pushFunc: func(_ context.Context, ref, _ string) (string, error) {
			mu.Lock()
			*pushed = append(*pushed, ref)
			mu.Unlock()

			return "sha256:" + strings.Repeat("b", 64), nil
		},
	}
}

func multiArchConfig() Config {
	return Config{
		Name:          "mealie",
		Version:       "v3.24.0",
		Registry:      "ghcr.io/cowdogmoo",
		Architectures: []string{"amd64", "arm64"},
		Targets:       []Target{{Type: "container", Registry: "ghcr.io/cowdogmoo", Tags: []string{"latest", "v3.24.0"}}},
	}
}

// TestMultiArchComponentBuildsSkipEveryReleaseTag pins the rule that a
// per-architecture image is a build component, not a release: it is tagged by
// architecture, and every release tag — declared by the template or requested with
// --tag — belongs on the manifest list that unites the architectures. Tagging them
// per architecture makes each architecture overwrite the other's :latest, and
// publishes a release tag that resolves to whichever architecture finished last.
func TestMultiArchComponentBuildsSkipEveryReleaseTag(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	opts := BuildOptions{Registry: "ghcr.io/cowdogmoo", Tags: []string{"v9.9.9", "v9.9"}}
	results, err := svc.ExecuteContainerBuild(context.Background(), multiArchConfig(), opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}

	for _, result := range results {
		if len(result.AdditionalRefs) != 0 {
			t.Errorf("architecture %q carries additional refs %v, want none", result.Architecture, result.AdditionalRefs)
		}
	}

	sort.Strings(built)
	want := []string{"ghcr.io/cowdogmoo/mealie:amd64", "ghcr.io/cowdogmoo/mealie:arm64"}
	if !reflect.DeepEqual(built, want) {
		t.Errorf("tagged %v, want only the per-architecture references %v", built, want)
	}
}

// TestPushMultiArchPublishesManifestTags checks the release tags reach the registry
// as manifest lists spanning every architecture built.
func TestPushMultiArchPublishesManifestTags(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })
	cfg := multiArchConfig()
	opts := BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}
	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	wantManifests := []string{"ghcr.io/cowdogmoo/mealie:latest", "ghcr.io/cowdogmoo/mealie:v3.24.0"}
	if got := bldr.manifestNames(); !reflect.DeepEqual(got, wantManifests) {
		t.Errorf("manifest lists = %v, want %v", got, wantManifests)
	}

	for name, entries := range bldr.manifests {
		if len(entries) != 2 {
			t.Errorf("manifest %s has %d entries, want one per architecture", name, len(entries))
		}
	}

	sort.Strings(pushed)
	wantPushed := []string{"ghcr.io/cowdogmoo/mealie:amd64", "ghcr.io/cowdogmoo/mealie:arm64"}
	if !reflect.DeepEqual(pushed, wantPushed) {
		t.Errorf("pushed %v, want only the per-architecture images %v", pushed, wantPushed)
	}
}

// TestPushMultiArchDigestPublishesNoTags checks --push-digest stays tagless: it
// publishes neither an extra tag nor a manifest list naming one.
func TestPushMultiArchDigestPublishesNoTags(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })
	cfg := multiArchConfig()
	opts := BuildOptions{Registry: "ghcr.io/cowdogmoo", PushDigest: true}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}
	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if got := bldr.manifestNames(); len(got) != 0 {
		t.Errorf("manifest lists = %v, want none for a digest push", got)
	}
}

// TestPushMultiArchRefusesEmptyManifest checks a tag is never published as an
// index naming no architecture: leaving the old tag in place beats replacing it
// with an empty one.
func TestPushMultiArchRefusesEmptyManifest(t *testing.T) {
	// A registry answering with something the manifest layer cannot parse leaves
	// no architecture describable.
	bldr := &mockContainerBuilder{
		pushFunc: func(_ context.Context, _, _ string) (string, error) {
			return "not-a-digest", nil
		},
	}
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	results := []BuildResult{
		{ImageRef: "ghcr.io/cowdogmoo/mealie:amd64", Platform: "linux/amd64"},
		{ImageRef: "ghcr.io/cowdogmoo/mealie:arm64", Platform: "linux/arm64"},
	}

	err := svc.Push(context.Background(), multiArchConfig(), results, BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true})
	if err == nil {
		t.Fatal("expected an error rather than an empty manifest list")
	}
	if !strings.Contains(err.Error(), "manifests create") {
		t.Errorf("error = %v, want it to point at the recovery command", err)
	}
	if got := bldr.manifestNames(); len(got) != 0 {
		t.Errorf("published %v, want no manifest at all", got)
	}
}

// TestPushMultiArchRefusesPartialManifest checks a manifest is never published
// while missing an architecture: a release tag that silently covers one of two
// architectures is as wrong as one covering none.
func TestPushMultiArchRefusesPartialManifest(t *testing.T) {
	var describable atomic.Bool
	bldr := &mockContainerBuilder{
		pushFunc: func(_ context.Context, _, _ string) (string, error) {
			// Only the first architecture pushed comes back describable.
			if describable.CompareAndSwap(false, true) {
				return "sha256:" + strings.Repeat("d", 64), nil
			}

			return "not-a-digest", nil
		},
	}
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	results := []BuildResult{
		{ImageRef: "ghcr.io/cowdogmoo/mealie:amd64", Platform: "linux/amd64"},
		{ImageRef: "ghcr.io/cowdogmoo/mealie:arm64", Platform: "linux/arm64"},
	}

	err := svc.Push(context.Background(), multiArchConfig(), results, BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true})
	if err == nil {
		t.Fatal("expected an error rather than a manifest missing an architecture")
	}
	if !strings.Contains(err.Error(), "1 of 2 architectures") {
		t.Errorf("error = %v, want it to report how many architectures were describable", err)
	}
	if got := bldr.manifestNames(); len(got) != 0 {
		t.Errorf("published %v, want no manifest at all", got)
	}
}

// TestPushMultiArchReportsManifestFailure checks a registry rejecting the
// manifest fails the push and names the tag that could not be published,
// instead of reporting a successful build whose release tag does not exist.
func TestPushMultiArchReportsManifestFailure(t *testing.T) {
	bldr := &mockContainerBuilder{
		pushFunc: func(_ context.Context, _, _ string) (string, error) {
			return "sha256:" + strings.Repeat("e", 64), nil
		},
		createManifestFunc: func(manifestName string, _ []manifests.ManifestEntry) error {
			return fmt.Errorf("registry rejected %s", manifestName)
		},
	}
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	results := []BuildResult{
		{ImageRef: "ghcr.io/cowdogmoo/mealie:amd64", Platform: "linux/amd64"},
		{ImageRef: "ghcr.io/cowdogmoo/mealie:arm64", Platform: "linux/arm64"},
	}

	err := svc.Push(context.Background(), multiArchConfig(), results, BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true})
	if err == nil {
		t.Fatal("expected an error when the manifest cannot be published")
	}
	if !strings.Contains(err.Error(), "ghcr.io/cowdogmoo/mealie:v3.24.0") {
		t.Errorf("error = %v, want it to name the tag that failed", err)
	}
}
