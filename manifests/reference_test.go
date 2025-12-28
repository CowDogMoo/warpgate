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

package manifests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildImageReference(t *testing.T) {
	tests := []struct {
		name     string
		opts     ReferenceOptions
		expected string
	}{
		{
			name: "with namespace",
			opts: ReferenceOptions{
				Registry:     "ghcr.io",
				Namespace:    "cowdogmoo",
				ImageName:    "attack-box",
				Tag:          "latest",
				Architecture: "amd64",
			},
			expected: "ghcr.io/cowdogmoo/attack-box-amd64:latest",
		},
		{
			name: "without namespace",
			opts: ReferenceOptions{
				Registry:     "ghcr.io",
				Namespace:    "",
				ImageName:    "sliver",
				Tag:          "v1.0.0",
				Architecture: "arm64",
			},
			expected: "ghcr.io/sliver-arm64:v1.0.0",
		},
		{
			name: "with arm variant",
			opts: ReferenceOptions{
				Registry:     "docker.io",
				Namespace:    "myorg",
				ImageName:    "test",
				Tag:          "dev",
				Architecture: "arm/v7",
			},
			expected: "docker.io/myorg/test-arm/v7:dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build image reference
			ref := BuildImageReference(tt.opts)
			assert.Equal(t, tt.expected, ref)
		})
	}
}

func TestBuildManifestReference(t *testing.T) {
	tests := []struct {
		name      string
		registry  string
		namespace string
		imageName string
		tag       string
		expected  string
	}{
		{
			name:      "with namespace",
			registry:  "ghcr.io",
			namespace: "cowdogmoo",
			imageName: "attack-box",
			tag:       "latest",
			expected:  "ghcr.io/cowdogmoo/attack-box:latest",
		},
		{
			name:      "without namespace",
			registry:  "ghcr.io",
			namespace: "",
			imageName: "sliver",
			tag:       "v1.0.0",
			expected:  "ghcr.io/sliver:v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build manifest reference
			ref := BuildManifestReference(tt.registry, tt.namespace, tt.imageName, tt.tag)
			assert.Equal(t, tt.expected, ref)
		})
	}
}
