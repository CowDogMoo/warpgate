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
	"runtime"

	"github.com/cowdogmoo/warpgate/v3/config"
	"github.com/cowdogmoo/warpgate/v3/logging"
)

// A BuildStrategy represents the strategy to use for building images.
type BuildStrategy int

const (
	// NativeBuild indicates a native build on the same architecture
	NativeBuild BuildStrategy = iota
	// CrossCompile indicates a cross-compilation using QEMU
	CrossCompile
)

// StrategyDetector determines the best build strategy for a given architecture
type StrategyDetector struct {
	hostArch     string
	hostOS       string
	globalConfig *config.Config
}

// NewStrategyDetector creates a new build strategy detector
func NewStrategyDetector() (*StrategyDetector, error) {
	cfg, err := config.Load()
	if err != nil && !config.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}

	return &StrategyDetector{
		hostArch:     runtime.GOARCH,
		hostOS:       runtime.GOOS,
		globalConfig: cfg,
	}, nil
}

// DetectStrategy determines the build strategy for a target architecture
func (sd *StrategyDetector) DetectStrategy(ctx context.Context, targetArch string) (BuildStrategy, string) {
	// Normalize architecture names
	normalizedTarget := normalizeArch(targetArch)
	normalizedHost := normalizeArch(sd.hostArch)

	if normalizedTarget == normalizedHost {
		logging.InfoContext(ctx, "Using native build for %s (host arch: %s)", targetArch, sd.hostArch)
		return NativeBuild, fmt.Sprintf("Native build on %s", sd.hostArch)
	}

	logging.WarnContext(ctx, "Cross-compiling for %s on %s host - this will use QEMU emulation and may be slower", targetArch, sd.hostArch)
	return CrossCompile, fmt.Sprintf("Cross-compile from %s to %s using QEMU", sd.hostArch, targetArch)
}

// GetOptimalRunnerForArch returns the recommended CI/CD runner for an architecture
func (sd *StrategyDetector) GetOptimalRunnerForArch(arch string) string {
	switch normalizeArch(arch) {
	case "amd64", "x86_64":
		return "ubuntu-latest" // or ubuntu-24.04
	case "arm64", "aarch64":
		return "ubuntu-24.04-arm"
	default:
		return "ubuntu-latest" // Default fallback
	}
}

// ShouldWarnSlowBuild determines if we should warn about slow build times
func (sd *StrategyDetector) ShouldWarnSlowBuild(ctx context.Context, targetArch string) bool {
	strategy, _ := sd.DetectStrategy(ctx, targetArch)
	return strategy == CrossCompile
}

// normalizeArch normalizes architecture names to a standard form
func normalizeArch(arch string) string {
	switch arch {
	case "amd64", "x86_64", "x64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	case "arm", "armv7", "armv7l":
		return "arm"
	case "386", "i386", "i686":
		return "386"
	default:
		return arch
	}
}

// EstimateBuildTime estimates relative build time based on strategy
func (sd *StrategyDetector) EstimateBuildTime(ctx context.Context, targetArch string, baselineMinutes int) (int, string) {
	strategy, _ := sd.DetectStrategy(ctx, targetArch)

	switch strategy {
	case NativeBuild:
		return baselineMinutes, "native speed"
	case CrossCompile:
		// QEMU cross-compilation slowdown from config
		slowdownFactor := sd.globalConfig.Build.QEMUSlowdownFactor
		slowMinutes := baselineMinutes * slowdownFactor
		return slowMinutes, fmt.Sprintf("~%dx slower due to QEMU emulation", slowdownFactor)
	default:
		return baselineMinutes, "unknown"
	}
}

// RecommendParallelism suggests how many parallel builds to run
func (sd *StrategyDetector) RecommendParallelism(ctx context.Context, architectures []string) int {
	// Count how many are native vs cross-compile
	nativeCount := 0
	crossCount := 0

	for _, arch := range architectures {
		strategy, _ := sd.DetectStrategy(ctx, arch)
		if strategy == NativeBuild {
			nativeCount++
		} else {
			crossCount++
		}
	}

	// Conservative parallelism to avoid resource exhaustion
	// Native builds can be more parallel since they're faster
	if crossCount > 0 {
		// If we have cross-compilation, limit parallelism (from config)
		return sd.globalConfig.Build.ParallelismLimit
	}

	// All native builds can be more aggressive
	cpus := runtime.NumCPU()
	if nativeCount <= cpus {
		return nativeCount
	}

	// Use configured CPU fraction (from config)
	return int(float64(cpus) * sd.globalConfig.Build.CPUFraction)
}

// GetBuildMatrix returns the build matrix for CI/CD
func (sd *StrategyDetector) GetBuildMatrix(ctx context.Context, architectures []string) []BuildMatrixEntry {
	var matrix []BuildMatrixEntry

	for _, arch := range architectures {
		strategy, reason := sd.DetectStrategy(ctx, arch)
		runner := sd.GetOptimalRunnerForArch(arch)
		estimatedMinutes, speedNote := sd.EstimateBuildTime(ctx, arch, 10) // 10 min baseline

		matrix = append(matrix, BuildMatrixEntry{
			Architecture:     arch,
			Runner:           runner,
			Strategy:         strategy,
			StrategyReason:   reason,
			EstimatedMinutes: estimatedMinutes,
			SpeedNote:        speedNote,
		})
	}

	return matrix
}

// A BuildMatrixEntry represents a single entry in the build matrix.
type BuildMatrixEntry struct {
	Architecture     string
	Runner           string
	Strategy         BuildStrategy
	StrategyReason   string
	EstimatedMinutes int
	SpeedNote        string
}

// String returns a human-readable representation of the build strategy
func (bs BuildStrategy) String() string {
	switch bs {
	case NativeBuild:
		return "native"
	case CrossCompile:
		return "cross-compile"
	default:
		return "unknown"
	}
}
