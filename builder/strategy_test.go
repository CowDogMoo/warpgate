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
	"testing"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/stretchr/testify/assert"
)

func newTestStrategyDetector(hostArch string, cfg *config.Config) *StrategyDetector {
	if cfg == nil {
		cfg = &config.Config{
			Build: config.BuildConfig{
				QEMUSlowdownFactor: 3,
				ParallelismLimit:   2,
				CPUFraction:        0.5,
			},
		}
	}
	return &StrategyDetector{
		hostArch:     hostArch,
		hostOS:       "linux",
		globalConfig: cfg,
	}
}

func TestNormalizeArch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"amd64 passthrough", "amd64", "amd64"},
		{"x86_64 to amd64", "x86_64", "amd64"},
		{"x64 to amd64", "x64", "amd64"},
		{"arm64 passthrough", "arm64", "arm64"},
		{"aarch64 to arm64", "aarch64", "arm64"},
		{"arm passthrough", "arm", "arm"},
		{"armv7 to arm", "armv7", "arm"},
		{"armv7l to arm", "armv7l", "arm"},
		{"386 passthrough", "386", "386"},
		{"i386 to 386", "i386", "386"},
		{"i686 to 386", "i686", "386"},
		{"unknown arch passes through", "riscv64", "riscv64"},
		{"empty string passes through", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeArch(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectStrategy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("native build when architectures match", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		strategy, reason := sd.DetectStrategy(ctx, "amd64")
		assert.Equal(t, NativeBuild, strategy)
		assert.Contains(t, reason, "Native build")
	})

	t.Run("native build with equivalent architectures", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		strategy, _ := sd.DetectStrategy(ctx, "x86_64")
		assert.Equal(t, NativeBuild, strategy)
	})

	t.Run("cross compile when architectures differ", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		strategy, reason := sd.DetectStrategy(ctx, "arm64")
		assert.Equal(t, CrossCompile, strategy)
		assert.Contains(t, reason, "Cross-compile")
		assert.Contains(t, reason, "QEMU")
	})

	t.Run("cross compile arm64 host to amd64 target", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("arm64", nil)
		strategy, _ := sd.DetectStrategy(ctx, "amd64")
		assert.Equal(t, CrossCompile, strategy)
	})
}

func TestGetOptimalRunnerForArch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arch string
		want string
	}{
		{"amd64", "amd64", "ubuntu-latest"},
		{"x86_64", "x86_64", "ubuntu-latest"},
		{"arm64", "arm64", "ubuntu-24.04-arm"},
		{"aarch64", "aarch64", "ubuntu-24.04-arm"},
		{"unknown fallback", "riscv64", "ubuntu-latest"},
	}

	sd := newTestStrategyDetector("amd64", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sd.GetOptimalRunnerForArch(tt.arch)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldWarnSlowBuild(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("no warning for native build", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		assert.False(t, sd.ShouldWarnSlowBuild(ctx, "amd64"))
	})

	t.Run("warns for cross compile", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		assert.True(t, sd.ShouldWarnSlowBuild(ctx, "arm64"))
	})
}

func TestEstimateBuildTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("native build returns baseline", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		minutes, note := sd.EstimateBuildTime(ctx, "amd64", 10)
		assert.Equal(t, 10, minutes)
		assert.Equal(t, "native speed", note)
	})

	t.Run("cross compile applies slowdown factor", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Build: config.BuildConfig{
				QEMUSlowdownFactor: 5,
				ParallelismLimit:   2,
				CPUFraction:        0.5,
			},
		}
		sd := newTestStrategyDetector("amd64", cfg)
		minutes, note := sd.EstimateBuildTime(ctx, "arm64", 10)
		assert.Equal(t, 50, minutes)
		assert.Contains(t, note, "5x slower")
	})
}

func TestRecommendParallelism(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns parallelism limit with cross compile", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Build: config.BuildConfig{
				QEMUSlowdownFactor: 3,
				ParallelismLimit:   2,
				CPUFraction:        0.5,
			},
		}
		sd := newTestStrategyDetector("amd64", cfg)
		result := sd.RecommendParallelism(ctx, []string{"amd64", "arm64"})
		assert.Equal(t, 2, result)
	})

	t.Run("all native uses arch count when <= CPUs", func(t *testing.T) {
		t.Parallel()
		sd := newTestStrategyDetector("amd64", nil)
		result := sd.RecommendParallelism(ctx, []string{"amd64"})
		assert.Equal(t, 1, result)
	})
}

func TestGetBuildMatrix(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sd := newTestStrategyDetector("amd64", nil)
	matrix := sd.GetBuildMatrix(ctx, []string{"amd64", "arm64"})

	assert.Len(t, matrix, 2)

	// amd64 should be native
	assert.Equal(t, "amd64", matrix[0].Architecture)
	assert.Equal(t, NativeBuild, matrix[0].Strategy)
	assert.Equal(t, "ubuntu-latest", matrix[0].Runner)

	// arm64 should be cross-compile
	assert.Equal(t, "arm64", matrix[1].Architecture)
	assert.Equal(t, CrossCompile, matrix[1].Strategy)
	assert.Equal(t, "ubuntu-24.04-arm", matrix[1].Runner)
}

func TestBuildStrategyString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy BuildStrategy
		want     string
	}{
		{"native", NativeBuild, "native"},
		{"cross-compile", CrossCompile, "cross-compile"},
		{"unknown", BuildStrategy(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.strategy.String())
		})
	}
}
