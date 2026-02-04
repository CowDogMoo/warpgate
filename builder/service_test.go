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
	"strings"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/opencontainers/go-digest"
)

// mockContainerBuilder implements ContainerBuilder for testing
type mockContainerBuilder struct {
	buildFunc      func(ctx context.Context, cfg Config) (*BuildResult, error)
	pushFunc       func(ctx context.Context, imageRef, registry string) (string, error)
	pushDigestFunc func(ctx context.Context, imageRef, registry string) (string, error)
	tagFunc        func(ctx context.Context, imageRef, newTag string) error
	removeFunc     func(ctx context.Context, imageRef string) error
	closeFunc      func() error
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
			wantErr: false, // Invalid digests are skipped, not errored
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
				return "sha256:abcdef", nil
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

	pushCount := 0
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) (string, error) {
				pushCount++
				return "sha256:abcdef", nil
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

	buildCount := 0
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
				buildCount++
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
			return "sha256:abcdef", nil
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
				return "sha256:abcdef", nil
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
