/*
Copyright © 2024 Jayson Grace <jayson.e.grace@gmail.com>

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
)

// Builder is the main interface for image builders
type Builder interface {
	// Build creates an image from the given configuration
	Build(ctx context.Context, config Config) (*BuildResult, error)

	// Close cleans up any resources used by the builder
	Close() error
}

// ContainerBuilder builds container images
type ContainerBuilder interface {
	Builder

	// Push pushes the built image to a registry
	Push(ctx context.Context, imageRef, registry string) error

	// Tag adds additional tags to an image
	Tag(ctx context.Context, imageRef, newTag string) error

	// Remove removes an image from local storage
	Remove(ctx context.Context, imageRef string) error
}

// AMIBuilder builds AWS AMIs
type AMIBuilder interface {
	Builder

	// Share shares the AMI with other AWS accounts
	Share(ctx context.Context, amiID string, accountIDs []string) error

	// Copy copies an AMI to another region
	Copy(ctx context.Context, amiID, sourceRegion, destRegion string) (string, error)

	// Deregister deregisters an AMI
	Deregister(ctx context.Context, amiID, region string) error
}
