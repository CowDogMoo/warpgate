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
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/globalconfig"
	"github.com/opencontainers/go-digest"
)

// mockContainerBuilder implements ContainerBuilder for testing
type mockContainerBuilder struct {
	buildFunc  func(ctx context.Context, cfg Config) (*BuildResult, error)
	pushFunc   func(ctx context.Context, imageRef, registry string) error
	tagFunc    func(ctx context.Context, imageRef, newTag string) error
	removeFunc func(ctx context.Context, imageRef string) error
	closeFunc  func() error
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

func (m *mockContainerBuilder) Push(ctx context.Context, imageRef, registry string) error {
	if m.pushFunc != nil {
		return m.pushFunc(ctx, imageRef, registry)
	}
	return nil
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
	cfg := &globalconfig.Config{}

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{}, nil
	}
	autoSelectCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{}, nil
	}

	service := NewBuildService(cfg, buildKitCreator, autoSelectCreator)

	if service == nil {
		t.Fatal("NewBuildService() returned nil")
	}

	if service.globalConfig != cfg {
		t.Error("NewBuildService() config not set correctly")
	}
}

func TestDetermineTargetType(t *testing.T) {
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
			got, err := CreateManifestEntries(tt.results)
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
	results := []BuildResult{
		{
			ImageRef:     "test-image:arm64v8",
			Architecture: "arm64",
			Platform:     "linux/arm64/v8",
			Digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
	}

	got, err := CreateManifestEntries(results)
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
	cfg := &globalconfig.Config{}

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

	service := NewBuildService(cfg, buildKitCreator, nil)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	buildOpts := BuildOptions{
		BuilderType: "buildkit",
	}

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
	cfg := &globalconfig.Config{}

	expectedErr := fmt.Errorf("build failed")
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			buildFunc: func(ctx context.Context, cfg Config) (*BuildResult, error) {
				return nil, expectedErr
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator, nil)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	buildOpts := BuildOptions{
		BuilderType: "buildkit",
	}

	ctx := context.Background()
	_, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err == nil {
		t.Error("ExecuteContainerBuild() expected error, got nil")
	}
}

func TestBuildService_ExecuteContainerBuild_BuilderCreationError(t *testing.T) {
	cfg := &globalconfig.Config{}

	expectedErr := fmt.Errorf("builder creation failed")
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return nil, expectedErr
	}

	service := NewBuildService(cfg, buildKitCreator, nil)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	buildOpts := BuildOptions{
		BuilderType: "buildkit",
	}

	ctx := context.Background()
	_, err := service.ExecuteContainerBuild(ctx, buildConfig, buildOpts)

	if err == nil {
		t.Error("ExecuteContainerBuild() expected error, got nil")
	}
}

func TestBuildService_SelectContainerBuilder(t *testing.T) {
	tests := []struct {
		name         string
		globalCfg    *globalconfig.Config
		opts         BuildOptions
		wantErr      bool
		builderCalls int
	}{
		{
			name: "select buildkit builder",
			globalCfg: &globalconfig.Config{
				Build: globalconfig.BuildConfig{
					BuilderType: "buildkit",
				},
			},
			opts:         BuildOptions{},
			wantErr:      false,
			builderCalls: 1,
		},
		{
			name: "CLI override builder type",
			globalCfg: &globalconfig.Config{
				Build: globalconfig.BuildConfig{
					BuilderType: "buildkit",
				},
			},
			opts: BuildOptions{
				BuilderType: "auto",
			},
			wantErr:      false,
			builderCalls: 1,
		},
		{
			name: "invalid builder type",
			globalCfg: &globalconfig.Config{
				Build: globalconfig.BuildConfig{
					BuilderType: "invalid",
				},
			},
			opts:    BuildOptions{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildKitCalls := 0

			buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
				buildKitCalls++
				return &mockContainerBuilder{}, nil
			}

			autoSelectCreator := func(ctx context.Context) (ContainerBuilder, error) {
				return &mockContainerBuilder{}, nil
			}

			service := NewBuildService(tt.globalCfg, buildKitCreator, autoSelectCreator)

			ctx := context.Background()
			builder, err := service.selectContainerBuilder(ctx, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("selectContainerBuilder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && builder != nil {
				_ = builder.Close()
			}
		})
	}
}

func TestManifestEntry_DigestParsing(t *testing.T) {
	result := BuildResult{
		ImageRef:     "test-image:amd64",
		Architecture: "amd64",
		Platform:     "linux/amd64",
		Digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	entries, err := CreateManifestEntries([]BuildResult{result})
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
	cfg := &globalconfig.Config{}

	service := NewBuildService(cfg, nil, nil)

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
	cfg := &globalconfig.Config{}

	pushCalled := false
	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{
			pushFunc: func(ctx context.Context, imageRef, registry string) error {
				pushCalled = true
				if registry != "ghcr.io/myorg" {
					return fmt.Errorf("unexpected registry: %s", registry)
				}
				return nil
			},
		}, nil
	}

	service := NewBuildService(cfg, buildKitCreator, nil)

	buildConfig := &Config{
		Name:          "test",
		Architectures: []string{"amd64"},
	}

	result := BuildResult{
		ImageRef:     "test-image:latest",
		Architecture: "amd64",
	}

	buildOpts := BuildOptions{
		Registry:    "ghcr.io/myorg",
		BuilderType: "buildkit",
	}

	ctx := context.Background()
	err := service.pushSingleArch(ctx, buildConfig, result, &mockContainerBuilder{
		pushFunc: func(ctx context.Context, imageRef, registry string) error {
			pushCalled = true
			return nil
		},
	}, buildOpts)

	if err != nil {
		t.Errorf("pushSingleArch() error = %v", err)
	}

	if !pushCalled {
		t.Error("pushSingleArch() did not call Push")
	}
}

func TestBuildService_ApplyOverridesBeforeBuild(t *testing.T) {
	cfg := &globalconfig.Config{}

	buildKitCreator := func(ctx context.Context) (ContainerBuilder, error) {
		return &mockContainerBuilder{}, nil
	}

	service := NewBuildService(cfg, buildKitCreator, nil)

	buildConfig := Config{
		Name:          "test",
		Architectures: []string{},
		Registry:      "",
	}

	buildOpts := BuildOptions{
		Architectures: []string{"amd64", "arm64"},
		Registry:      "ghcr.io/myorg",
		BuilderType:   "buildkit",
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
