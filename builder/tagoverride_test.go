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
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
)

// overrideConfig is the configuration a template supplies, before any CLI
// override reaches it: the version is the template's own, and the container
// target declares one floating tag.
func overrideConfig(architectures ...string) Config {
	return Config{
		Name:          "mealie",
		Version:       "latest",
		Registry:      "ghcr.io/cowdogmoo",
		Architectures: architectures,
		Targets:       []Target{{Type: "container", Registry: "ghcr.io/cowdogmoo", Tags: []string{"stable"}}},
	}
}

// TestPushMultiArchAppliesTagOverride pins what --tag means for a multi-arch
// release: it replaces the version the release is named after, and the tags the
// target declares still publish alongside it.
//
// The configuration handed to Push is deliberately the unresolved one, because
// that is what cmd/warpgate passes: ExecuteContainerBuild takes its Config by
// value and resolves its own copy, so a test that pushes a resolved config
// passes while the release still goes out under the template's version.
func TestPushMultiArchAppliesTagOverride(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	cfg := overrideConfig("amd64", "arm64")
	opts := BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true, Tags: []string{"v9.9.9"}}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}

	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	want := []string{"ghcr.io/cowdogmoo/mealie:stable", "ghcr.io/cowdogmoo/mealie:v9.9.9"}
	if got := bldr.manifestNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("manifest lists = %v, want %v", got, want)
	}
}

// TestPushMultiArchAppliesRegistryOverride checks --registry reaches the
// manifest names by the same route, from a configuration that names no registry
// at all. This used to be covered by a fallback inside publishManifestTags;
// resolving the overrides at push time is what replaced it.
func TestPushMultiArchAppliesRegistryOverride(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	cfg := Config{
		Name:          "mealie",
		Version:       "v3.24.0",
		Architectures: []string{"amd64", "arm64"},
		Targets:       []Target{{Type: "container", Tags: []string{"stable"}}},
	}
	opts := BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}

	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	want := []string{"ghcr.io/cowdogmoo/mealie:stable", "ghcr.io/cowdogmoo/mealie:v3.24.0"}
	if got := bldr.manifestNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("manifest lists = %v, want %v", got, want)
	}
}

// TestPushSingleArchUsesOverriddenTag confirms the single-architecture path was
// never affected: it pushes the reference the build already resolved, so --tag
// reached it before this change and still does, and it publishes no manifest
// list.
func TestPushSingleArchUsesOverriddenTag(t *testing.T) {
	var mu sync.Mutex
	var built, pushed []string

	bldr := recordingBuilder(&built, &pushed, &mu)
	svc := NewBuildService(nil, func(_ context.Context) (ContainerBuilder, error) { return bldr, nil })

	cfg := overrideConfig("amd64")
	opts := BuildOptions{Registry: "ghcr.io/cowdogmoo", Push: true, Tags: []string{"v9.9.9"}}

	results, err := svc.ExecuteContainerBuild(context.Background(), cfg, opts)
	if err != nil {
		t.Fatalf("ExecuteContainerBuild() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ExecuteContainerBuild() produced %d results, want 1", len(results))
	}

	if err := svc.Push(context.Background(), cfg, results, opts); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	sort.Strings(pushed)
	want := []string{"ghcr.io/cowdogmoo/mealie:stable", "ghcr.io/cowdogmoo/mealie:v9.9.9"}
	if !reflect.DeepEqual(pushed, want) {
		t.Errorf("pushed %v, want %v", pushed, want)
	}
	if got := bldr.manifestNames(); len(got) != 0 {
		t.Errorf("published %v, want no manifest list for a single-architecture build", got)
	}
}

// TestApplyOverridesIsIdempotent guards the property the push path relies on:
// resolving a configuration that is already resolved has to leave it alone,
// because the build and the push each resolve their own copy.
func TestApplyOverridesIsIdempotent(t *testing.T) {
	globalCfg := &config.Config{
		Registry: config.RegistryConfig{Default: "ghcr.io"},
	}
	globalCfg.Build.DefaultArch = []string{"amd64"}

	tests := []struct {
		name string
		cfg  func() Config
		opts BuildOptions
	}{
		{
			name: "container overrides",
			cfg:  func() Config { return overrideConfig("amd64", "arm64") },
			opts: BuildOptions{
				TargetType:    "container",
				Architectures: []string{"arm64"},
				Registry:      "ghcr.io/cowdogmoo",
				Tags:          []string{"v9.9.9"},
				Labels:        map[string]string{"org.opencontainers.image.title": "mealie"},
				BuildArgs:     map[string]string{"BASE_IMAGE": "alpine"},
				NoCache:       true,
			},
		},
		{
			name: "ami overrides",
			cfg: func() Config {
				return Config{
					Name:    "attack-box",
					Version: "v1.0.0",
					Targets: []Target{{Type: "ami", Region: "us-east-1"}},
				}
			},
			opts: BuildOptions{Region: "us-west-2", InstanceType: "t3.large"},
		},
		{
			name: "defaults from the global config only",
			cfg:  func() Config { return Config{Name: "mealie", Version: "v1.0.0"} },
			opts: BuildOptions{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			once := tt.cfg()
			ApplyOverrides(ctx, &once, tt.opts, globalCfg)

			twice := tt.cfg()
			ApplyOverrides(ctx, &twice, tt.opts, globalCfg)
			ApplyOverrides(ctx, &twice, tt.opts, globalCfg)

			if !reflect.DeepEqual(once, twice) {
				t.Errorf("resolving twice produced %+v, want the same configuration as resolving once, %+v", twice, once)
			}
		})
	}
}
