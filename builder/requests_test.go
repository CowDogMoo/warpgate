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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBuildRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("creates requests for each architecture", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Name:          "myimage",
			Version:       "1.0.0",
			Architectures: []string{"amd64", "arm64"},
		}

		requests := CreateBuildRequests(ctx, config)

		require.Len(t, requests, 2)
		assert.Equal(t, "amd64", requests[0].Architecture)
		assert.Equal(t, "linux/amd64", requests[0].Platform)
		assert.Equal(t, "myimage:1.0.0", requests[0].Tag)

		assert.Equal(t, "arm64", requests[1].Architecture)
		assert.Equal(t, "linux/arm64", requests[1].Platform)
		assert.Equal(t, "myimage:1.0.0", requests[1].Tag)
	})

	t.Run("single architecture", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Name:          "single",
			Version:       "2.0.0",
			Architectures: []string{"amd64"},
		}

		requests := CreateBuildRequests(ctx, config)
		require.Len(t, requests, 1)
		assert.Equal(t, "single:2.0.0", requests[0].Tag)
	})

	t.Run("empty architectures returns no requests", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Name:          "empty",
			Version:       "1.0.0",
			Architectures: []string{},
		}

		requests := CreateBuildRequests(ctx, config)
		assert.Empty(t, requests)
	})

	t.Run("applies arch overrides when present", func(t *testing.T) {
		t.Parallel()
		overrideBase := BaseImage{Image: "alpine:3.18"}
		config := &Config{
			Name:          "overridden",
			Version:       "1.0.0",
			Architectures: []string{"arm64"},
			Base:          BaseImage{Image: "ubuntu:22.04"},
			ArchOverrides: map[string]ArchOverride{
				"arm64": {
					Base: &overrideBase,
				},
			},
		}

		requests := CreateBuildRequests(ctx, config)
		require.Len(t, requests, 1)
		assert.Equal(t, "alpine:3.18", requests[0].Config.Base.Image)
	})

	t.Run("no overrides preserves original config", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Name:          "no-override",
			Version:       "1.0.0",
			Architectures: []string{"amd64"},
			Base:          BaseImage{Image: "ubuntu:22.04"},
		}

		requests := CreateBuildRequests(ctx, config)
		require.Len(t, requests, 1)
		assert.Equal(t, "ubuntu:22.04", requests[0].Config.Base.Image)
	})
}

func TestApplyArchOverrides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("overrides base image", func(t *testing.T) {
		t.Parallel()
		overrideBase := BaseImage{Image: "alpine:3.18"}
		config := &Config{
			Base: BaseImage{Image: "ubuntu:22.04"},
		}

		ApplyArchOverrides(ctx, config, ArchOverride{
			Base: &overrideBase,
		}, "arm64")

		assert.Equal(t, "alpine:3.18", config.Base.Image)
	})

	t.Run("replaces provisioners by default", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Provisioners: []Provisioner{
				{Type: "shell", Inline: []string{"echo original"}},
			},
		}

		ApplyArchOverrides(ctx, config, ArchOverride{
			Provisioners: []Provisioner{
				{Type: "shell", Inline: []string{"echo replacement"}},
			},
		}, "arm64")

		require.Len(t, config.Provisioners, 1)
		assert.Equal(t, "echo replacement", config.Provisioners[0].Inline[0])
	})

	t.Run("appends provisioners when AppendProvisioners is true", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Provisioners: []Provisioner{
				{Type: "shell", Inline: []string{"echo original"}},
			},
		}

		ApplyArchOverrides(ctx, config, ArchOverride{
			Provisioners: []Provisioner{
				{Type: "shell", Inline: []string{"echo appended"}},
			},
			AppendProvisioners: true,
		}, "arm64")

		require.Len(t, config.Provisioners, 2)
		assert.Equal(t, "echo original", config.Provisioners[0].Inline[0])
		assert.Equal(t, "echo appended", config.Provisioners[1].Inline[0])
	})

	t.Run("no-op when override has no base or provisioners", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Base: BaseImage{Image: "ubuntu:22.04"},
		}

		ApplyArchOverrides(ctx, config, ArchOverride{}, "arm64")

		assert.Equal(t, "ubuntu:22.04", config.Base.Image)
	})
}

func TestExtractArchitecturesFromTargets(t *testing.T) {
	t.Parallel()

	t.Run("extracts architectures from platforms", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Targets: []Target{
				{Platforms: []string{"linux/amd64", "linux/arm64"}},
			},
		}

		archs := ExtractArchitecturesFromTargets(config)
		sort.Strings(archs)
		assert.Equal(t, []string{"amd64", "arm64"}, archs)
	})

	t.Run("deduplicates architectures", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Targets: []Target{
				{Platforms: []string{"linux/amd64"}},
				{Platforms: []string{"linux/amd64"}},
			},
		}

		archs := ExtractArchitecturesFromTargets(config)
		assert.Len(t, archs, 1)
		assert.Equal(t, "amd64", archs[0])
	})

	t.Run("handles platform with variant", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Targets: []Target{
				{Platforms: []string{"linux/arm/v7"}},
			},
		}

		archs := ExtractArchitecturesFromTargets(config)
		assert.Contains(t, archs, "arm")
	})

	t.Run("returns empty for no targets", func(t *testing.T) {
		t.Parallel()
		config := &Config{}
		archs := ExtractArchitecturesFromTargets(config)
		assert.Empty(t, archs)
	})

	t.Run("skips malformed platform strings", func(t *testing.T) {
		t.Parallel()
		config := &Config{
			Targets: []Target{
				{Platforms: []string{"noslash", "linux/amd64"}},
			},
		}

		archs := ExtractArchitecturesFromTargets(config)
		assert.Len(t, archs, 1)
		assert.Contains(t, archs, "amd64")
	})
}
