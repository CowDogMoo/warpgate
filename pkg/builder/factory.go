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
	"strings"
)

// BuilderType represents the type of builder to use
type BuilderType string

const (
	// BuilderTypeAuto automatically selects the best builder for the platform
	BuilderTypeAuto BuilderType = "auto"
	// BuilderTypeBuildKit uses BuildKit for building images
	BuilderTypeBuildKit BuilderType = "buildkit"
	// BuilderTypeBuildah uses Buildah for building images (Linux only)
	BuilderTypeBuildah BuilderType = "buildah"
)

// BuilderCreatorFunc creates a ContainerBuilder instance for a specific backend.
// It is called by the BuilderFactory to instantiate builders based on configuration.
// The context can be used for initialization and resource cleanup.
type BuilderCreatorFunc func(ctx context.Context) (ContainerBuilder, error)

// BuilderFactory creates builder instances based on configuration.
// It supports multiple builder backends (BuildKit, Buildah) and can automatically
// select the best builder for the current platform. The factory pattern allows
// platform-specific builder creation while maintaining a consistent interface.
type BuilderFactory struct {
	builderType       BuilderType
	buildKitCreator   BuilderCreatorFunc
	buildahCreator    BuilderCreatorFunc
	autoSelectCreator BuilderCreatorFunc
}

// NewBuilderFactory creates a new builder factory with the specified type and creator functions.
// The builderType parameter accepts "auto", "buildkit", or "buildah" (case-insensitive).
// Creator functions are provided for each backend and should return nil if unavailable on the platform.
func NewBuilderFactory(builderType string, buildKitCreator, buildahCreator, autoSelectCreator BuilderCreatorFunc) *BuilderFactory {
	// Normalize builder type
	normalizedType := strings.ToLower(strings.TrimSpace(builderType))

	var bt BuilderType
	switch normalizedType {
	case "buildkit":
		bt = BuilderTypeBuildKit
	case "buildah":
		bt = BuilderTypeBuildah
	case "auto", "":
		bt = BuilderTypeAuto
	default:
		bt = BuilderTypeAuto
	}

	return &BuilderFactory{
		builderType:       bt,
		buildKitCreator:   buildKitCreator,
		buildahCreator:    buildahCreator,
		autoSelectCreator: autoSelectCreator,
	}
}

// CreateContainerBuilder creates a ContainerBuilder instance based on the factory configuration
func (f *BuilderFactory) CreateContainerBuilder(ctx context.Context) (ContainerBuilder, error) {
	switch f.builderType {
	case BuilderTypeBuildKit:
		if f.buildKitCreator == nil {
			return nil, fmt.Errorf("BuildKit creator not provided")
		}
		return f.buildKitCreator(ctx)
	case BuilderTypeBuildah:
		if f.buildahCreator == nil {
			return nil, fmt.Errorf("buildah creator not provided")
		}
		return f.buildahCreator(ctx)
	case BuilderTypeAuto:
		if f.autoSelectCreator == nil {
			return nil, fmt.Errorf("auto-select creator not provided")
		}
		return f.autoSelectCreator(ctx)
	default:
		return nil, fmt.Errorf("unsupported builder type: %s", f.builderType)
	}
}

// BuilderType returns the configured builder type
func (f *BuilderFactory) BuilderType() BuilderType {
	return f.builderType
}

// String returns a string representation of the builder type
func (bt BuilderType) String() string {
	return string(bt)
}

// ValidateBuilderType validates if a builder type string is valid
func ValidateBuilderType(builderType string) error {
	normalizedType := strings.ToLower(strings.TrimSpace(builderType))

	switch normalizedType {
	case "auto", "buildkit", "buildah", "":
		return nil
	default:
		return fmt.Errorf("invalid builder type: %s (supported: auto, buildkit, buildah)", builderType)
	}
}
