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
	"fmt"
	"strings"
)

// ReferenceOptions contains options for building image references
type ReferenceOptions struct {
	Registry     string
	Namespace    string
	ImageName    string
	Architecture string
	Tag          string
}

// BuildImageReference builds a full image reference for a specific architecture
func BuildImageReference(opts ReferenceOptions) string {
	parts := []string{opts.Registry}

	if opts.Namespace != "" {
		parts = append(parts, opts.Namespace)
	}

	imageName := fmt.Sprintf("%s-%s:%s", opts.ImageName, opts.Architecture, opts.Tag)
	parts = append(parts, imageName)

	return strings.Join(parts, "/")
}

// BuildManifestReference builds the full manifest reference
func BuildManifestReference(registry, namespace, imageName, tag string) string {
	parts := []string{registry}

	if namespace != "" {
		parts = append(parts, namespace)
	}

	manifestName := fmt.Sprintf("%s:%s", imageName, tag)
	parts = append(parts, manifestName)

	return strings.Join(parts, "/")
}
