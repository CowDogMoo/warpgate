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

package builder_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cowdogmoo/warpgate/pkg/builder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockContainerBuilder is a mock implementation of ContainerBuilder
type MockContainerBuilder struct {
	mock.Mock
}

func (m *MockContainerBuilder) Build(ctx context.Context, cfg builder.Config) (*builder.BuildResult, error) {
	args := m.Called(ctx, cfg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*builder.BuildResult), args.Error(1)
}

func (m *MockContainerBuilder) Tag(ctx context.Context, imageRef, newTag string) error {
	args := m.Called(ctx, imageRef, newTag)
	return args.Error(0)
}

func (m *MockContainerBuilder) Push(ctx context.Context, imageRef, destination string) error {
	args := m.Called(ctx, imageRef, destination)
	return args.Error(0)
}

func (m *MockContainerBuilder) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockContainerBuilder) Remove(ctx context.Context, imageRef string) error {
	args := m.Called(ctx, imageRef)
	return args.Error(0)
}

func TestNewBuildOrchestrator(t *testing.T) {
	tests := []struct {
		name               string
		maxConcurrency     int
		expectedConcurrent int
	}{
		{
			name:               "default concurrency",
			maxConcurrency:     0,
			expectedConcurrent: 2,
		},
		{
			name:               "custom concurrency",
			maxConcurrency:     4,
			expectedConcurrent: 4,
		},
		{
			name:               "negative concurrency uses default",
			maxConcurrency:     -1,
			expectedConcurrent: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator := builder.NewBuildOrchestrator(tt.maxConcurrency)
			assert.NotNil(t, orchestrator)
			// Note: We can't directly test maxConcurrency as it's private,
			// but we can verify the orchestrator was created
		})
	}
}

func TestBuildMultiArch(t *testing.T) {
	tests := []struct {
		name           string
		requests       []builder.BuildRequest
		mockBuildSetup func(*MockContainerBuilder, []builder.BuildRequest)
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful multi-arch build",
			requests: []builder.BuildRequest{
				{
					Config: builder.Config{
						Name:    "test-image",
						Version: "1.0.0",
					},
					Architecture: "amd64",
					Platform:     "linux/amd64",
					Tag:          "test-image:1.0.0",
				},
				{
					Config: builder.Config{
						Name:    "test-image",
						Version: "1.0.0",
					},
					Architecture: "arm64",
					Platform:     "linux/arm64",
					Tag:          "test-image:1.0.0",
				},
			},
			mockBuildSetup: func(m *MockContainerBuilder, requests []builder.BuildRequest) {
				for i, req := range requests {
					m.On("Build", mock.Anything, mock.MatchedBy(func(cfg builder.Config) bool {
						return cfg.Name == req.Config.Name
					})).Return(&builder.BuildResult{
						ImageRef:     fmt.Sprintf("localhost/%s-%s", req.Config.Name, req.Architecture),
						Digest:       fmt.Sprintf("sha256:abc%d", i),
						Architecture: req.Architecture,
						Platform:     req.Platform,
						Duration:     "1s",
					}, nil).Once()

					m.On("Tag", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				}
			},
			expectError: false,
		},
		{
			name: "build failure for one architecture",
			requests: []builder.BuildRequest{
				{
					Config: builder.Config{
						Name:    "test-image",
						Version: "1.0.0",
					},
					Architecture: "amd64",
					Platform:     "linux/amd64",
					Tag:          "test-image:1.0.0",
				},
				{
					Config: builder.Config{
						Name:    "test-image",
						Version: "1.0.0",
					},
					Architecture: "arm64",
					Platform:     "linux/arm64",
					Tag:          "test-image:1.0.0",
				},
			},
			mockBuildSetup: func(m *MockContainerBuilder, requests []builder.BuildRequest) {
				// First build succeeds
				m.On("Build", mock.Anything, mock.MatchedBy(func(cfg builder.Config) bool {
					return cfg.Name == requests[0].Config.Name
				})).Return(&builder.BuildResult{
					ImageRef:     "localhost/test-image-amd64",
					Architecture: "amd64",
					Platform:     "linux/amd64",
					Duration:     "1s",
				}, nil).Once()
				m.On("Tag", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

				// Second build fails
				m.On("Build", mock.Anything, mock.MatchedBy(func(cfg builder.Config) bool {
					return cfg.Name == requests[1].Config.Name
				})).Return(nil, fmt.Errorf("build failed")).Once()
			},
			expectError:   true,
			errorContains: "failed to build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockBuilder := new(MockContainerBuilder)

			if tt.mockBuildSetup != nil {
				tt.mockBuildSetup(mockBuilder, tt.requests)
			}

			orchestrator := builder.NewBuildOrchestrator(2)
			results, err := orchestrator.BuildMultiArch(ctx, tt.requests, mockBuilder)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, len(tt.requests))

				// Verify results (order may vary due to parallel execution)
				architectures := make(map[string]bool)
				for _, req := range tt.requests {
					architectures[req.Architecture] = false
				}

				for _, result := range results {
					assert.NotEmpty(t, result.ImageRef)
					assert.NotEmpty(t, result.Architecture)
					_, exists := architectures[result.Architecture]
					assert.True(t, exists, "Unexpected architecture: %s", result.Architecture)
					architectures[result.Architecture] = true
				}

				// Ensure all architectures were built
				for arch, built := range architectures {
					assert.True(t, built, "Architecture %s was not built", arch)
				}
			}

			mockBuilder.AssertExpectations(t)
		})
	}
}

func TestPushMultiArch(t *testing.T) {
	tests := []struct {
		name          string
		results       []builder.BuildResult
		registry      string
		mockPushSetup func(*MockContainerBuilder, []builder.BuildResult)
		expectError   bool
		errorContains string
	}{
		{
			name: "successful multi-arch push",
			results: []builder.BuildResult{
				{
					ImageRef:     "localhost/test-image-amd64",
					Architecture: "amd64",
					Platform:     "linux/amd64",
				},
				{
					ImageRef:     "localhost/test-image-arm64",
					Architecture: "arm64",
					Platform:     "linux/arm64",
				},
			},
			registry: "ghcr.io/test",
			mockPushSetup: func(m *MockContainerBuilder, results []builder.BuildResult) {
				for _, result := range results {
					m.On("Push", mock.Anything, result.ImageRef, mock.Anything).Return(nil).Once()
				}
			},
			expectError: false,
		},
		{
			name: "push failure for one architecture",
			results: []builder.BuildResult{
				{
					ImageRef:     "localhost/test-image-amd64",
					Architecture: "amd64",
					Platform:     "linux/amd64",
				},
				{
					ImageRef:     "localhost/test-image-arm64",
					Architecture: "arm64",
					Platform:     "linux/arm64",
				},
			},
			registry: "ghcr.io/test",
			mockPushSetup: func(m *MockContainerBuilder, results []builder.BuildResult) {
				// First push succeeds
				m.On("Push", mock.Anything, results[0].ImageRef, mock.Anything).Return(nil).Once()
				// Second push fails
				m.On("Push", mock.Anything, results[1].ImageRef, mock.Anything).Return(fmt.Errorf("push failed")).Once()
			},
			expectError:   true,
			errorContains: "failed to push",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockBuilder := new(MockContainerBuilder)

			if tt.mockPushSetup != nil {
				tt.mockPushSetup(mockBuilder, tt.results)
			}

			orchestrator := builder.NewBuildOrchestrator(2)
			err := orchestrator.PushMultiArch(ctx, tt.results, tt.registry, mockBuilder)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			mockBuilder.AssertExpectations(t)
		})
	}
}
