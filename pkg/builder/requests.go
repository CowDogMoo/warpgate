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
	"fmt"
	"strings"

	"github.com/cowdogmoo/warpgate/v3/pkg/logging"
)

// CreateBuildRequests creates build requests for each architecture, applying any arch-specific overrides.
func CreateBuildRequests(buildConfig *Config) []BuildRequest {
	requests := make([]BuildRequest, 0, len(buildConfig.Architectures))

	for _, arch := range buildConfig.Architectures {
		platform := fmt.Sprintf("linux/%s", arch)
		tag := fmt.Sprintf("%s:%s", buildConfig.Name, buildConfig.Version)

		// Create a copy of the config for this architecture
		archConfig := *buildConfig

		// Apply architecture-specific overrides if they exist
		if override, ok := buildConfig.ArchOverrides[arch]; ok {
			ApplyArchOverrides(&archConfig, override, arch)
		}

		requests = append(requests, BuildRequest{
			Config:       archConfig,
			Architecture: arch,
			Platform:     platform,
			Tag:          tag,
		})
	}

	return requests
}

// ApplyArchOverrides applies architecture-specific overrides to the build configuration.
func ApplyArchOverrides(archConfig *Config, override ArchOverride, arch string) {
	logging.Info("Applying architecture overrides for %s", arch)

	// Override base image if specified
	if override.Base != nil {
		archConfig.Base = *override.Base
	}

	// Override or append provisioners
	if len(override.Provisioners) > 0 {
		if override.AppendProvisioners {
			archConfig.Provisioners = append(archConfig.Provisioners, override.Provisioners...)
		} else {
			archConfig.Provisioners = override.Provisioners
		}
	}
}

// ExtractArchitecturesFromTargets extracts unique architectures from target platform specifications.
func ExtractArchitecturesFromTargets(buildConfig *Config) []string {
	archMap := make(map[string]bool)

	for _, target := range buildConfig.Targets {
		for _, platform := range target.Platforms {
			// Platform format is "os/arch" or "os/arch/variant"
			parts := strings.Split(platform, "/")
			if len(parts) >= 2 {
				arch := parts[1]
				archMap[arch] = true
			}
		}
	}

	// Convert map to slice
	archs := make([]string, 0, len(archMap))
	for arch := range archMap {
		archs = append(archs, arch)
	}

	return archs
}
