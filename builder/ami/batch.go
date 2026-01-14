/*
Copyright © 2026 Jayson Grace <jayson.e.grace@gmail.com>

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

// Package ami provides batch operations for AWS EC2 Image Builder resources.
//
// This file implements BatchOperations which provides optimized, concurrent
// AWS API operations with rate limiting. All batch methods are safe for
// concurrent use and employ a semaphore pattern to limit concurrent AWS
// API requests to 5, avoiding rate limiting errors.
//
// Key features:
//   - Concurrent operations with configurable rate limiting
//   - Context cancellation support for graceful shutdown
//   - Error aggregation for batch operations
//   - Resource existence checking
package ami

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
	ibtypes "github.com/aws/aws-sdk-go-v2/service/imagebuilder/types"
	"github.com/cowdogmoo/warpgate/v3/logging"
	"golang.org/x/sync/errgroup"
)

// BatchOps defines the interface for batch AWS API operations.
// This interface enables testing with mock implementations.
type BatchOps interface {
	BatchTagResources(ctx context.Context, resourceIDs []string, tags map[string]string) error
	BatchDeleteComponents(ctx context.Context, componentARNs []string) error
	BatchDescribeImages(ctx context.Context, imageIDs []string) ([]ec2types.Image, error)
	BatchGetComponentVersions(ctx context.Context, componentNames []string) (map[string][]string, error)
	BatchCheckResourceExistence(ctx context.Context, checks []ResourceCheck) map[string]bool
}

// Compile-time check that BatchOperations implements BatchOps
var _ BatchOps = (*BatchOperations)(nil)

// BatchOperations provides optimized batch AWS API operations
type BatchOperations struct {
	clients *AWSClients
}

// NewBatchOperations creates a new batch operations helper
func NewBatchOperations(clients *AWSClients) *BatchOperations {
	return &BatchOperations{
		clients: clients,
	}
}

// BatchTagResources tags multiple resources in a single API call where possible.
// EC2 CreateTags supports multiple resources in one call.
//
// BatchTagResources is safe for concurrent use.
func (bo *BatchOperations) BatchTagResources(ctx context.Context, resourceIDs []string, tags map[string]string) error {
	if len(resourceIDs) == 0 || len(tags) == 0 {
		return nil
	}

	// Convert tags to EC2 format
	ec2Tags := make([]ec2types.Tag, 0, len(tags))
	for k, v := range tags {
		ec2Tags = append(ec2Tags, ec2types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	input := &ec2.CreateTagsInput{
		Resources: resourceIDs,
		Tags:      ec2Tags,
	}

	_, err := bo.clients.EC2.CreateTags(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to batch tag resources: %w", err)
	}

	logging.InfoContext(ctx, "Tagged %d resources with %d tags", len(resourceIDs), len(tags))
	return nil
}

// BatchDeleteComponents deletes multiple components concurrently.
// Returns a combined error using errors.Join if any deletions failed.
//
// This method deliberately continues processing remaining components even when
// individual deletions fail, returning all errors at the end. This "best effort"
// approach is appropriate for cleanup operations where partial success is
// preferable to failing fast.
//
// BatchDeleteComponents is safe for concurrent use.
func (bo *BatchOperations) BatchDeleteComponents(ctx context.Context, componentARNs []string) error {
	if len(componentARNs) == 0 {
		return nil
	}

	var mu sync.Mutex
	var errs []error

	g, ctx := errgroup.WithContext(ctx)
	// Limit to 5 concurrent AWS API requests to avoid rate limiting
	semaphore := make(chan struct{}, 5)

	for _, arn := range componentARNs {
		arn := arn // capture loop variable
		g.Go(func() error {
			// Check for context cancellation before starting work
			if err := ctx.Err(); err != nil {
				return err
			}

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			input := &imagebuilder.DeleteComponentInput{
				ComponentBuildVersionArn: aws.String(arn),
			}

			_, err := bo.clients.ImageBuilder.DeleteComponent(ctx, input)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to delete component %s: %w", arn, err))
				mu.Unlock()
			}
			return nil // Continue processing remaining components on individual errors
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		errs = append(errs, fmt.Errorf("batch operation interrupted: %w", err))
	}

	if len(errs) > 0 {
		logging.WarnContext(ctx, "Batch delete completed with %d errors out of %d components", len(errs), len(componentARNs))
		return errors.Join(errs...)
	}

	logging.InfoContext(ctx, "Successfully deleted %d components", len(componentARNs))
	return nil
}

// BatchDescribeImages describes multiple AMIs in a single API call.
// EC2 DescribeImages supports up to 1000 image IDs; this method handles batching automatically.
//
// BatchDescribeImages is safe for concurrent use.
func (bo *BatchOperations) BatchDescribeImages(ctx context.Context, imageIDs []string) ([]ec2types.Image, error) {
	if len(imageIDs) == 0 {
		return nil, nil
	}

	const maxBatchSize = 100
	var allImages []ec2types.Image

	for i := 0; i < len(imageIDs); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(imageIDs) {
			end = len(imageIDs)
		}
		batch := imageIDs[i:end]

		input := &ec2.DescribeImagesInput{
			ImageIds: batch,
		}

		result, err := bo.clients.EC2.DescribeImages(ctx, input)
		if err != nil {
			return allImages, fmt.Errorf("failed to describe images batch: %w", err)
		}

		allImages = append(allImages, result.Images...)
	}

	return allImages, nil
}

// BatchGetComponentVersions fetches versions for multiple components concurrently.
// Uses errgroup for parallel fetching with rate limiting.
//
// BatchGetComponentVersions is safe for concurrent use.
func (bo *BatchOperations) BatchGetComponentVersions(ctx context.Context, componentNames []string) (map[string][]string, error) {
	if len(componentNames) == 0 {
		return nil, nil
	}

	var mu sync.Mutex
	results := make(map[string][]string)

	g, ctx := errgroup.WithContext(ctx)
	// Limit to 5 concurrent AWS API requests to avoid rate limiting
	semaphore := make(chan struct{}, 5)

	for _, name := range componentNames {
		name := name // capture loop variable
		g.Go(func() error {
			// Check for context cancellation before starting work
			if err := ctx.Err(); err != nil {
				return err
			}

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			input := &imagebuilder.ListComponentsInput{
				Filters: []ibtypes.Filter{
					{
						Name:   aws.String("name"),
						Values: []string{name},
					},
				},
			}

			result, err := bo.clients.ImageBuilder.ListComponents(ctx, input)
			if err != nil {
				return fmt.Errorf("failed to list components for %s: %w", name, err)
			}

			var versions []string
			for _, comp := range result.ComponentVersionList {
				if comp.Version != nil {
					versions = append(versions, *comp.Version)
				}
			}

			mu.Lock()
			results[name] = versions
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}

	return results, nil
}

// BatchCheckResourceExistence checks if multiple resources exist concurrently.
// Returns a map of resource names to their existence status.
//
// BatchCheckResourceExistence is safe for concurrent use.
func (bo *BatchOperations) BatchCheckResourceExistence(ctx context.Context, checks []ResourceCheck) map[string]bool {
	results := make(map[string]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit to 5 concurrent AWS API requests to avoid rate limiting
	semaphore := make(chan struct{}, 5)

	for _, check := range checks {
		check := check // capture loop variable
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Check for context cancellation before starting work
			if ctx.Err() != nil {
				return
			}

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			exists := bo.checkResourceExists(ctx, check)

			mu.Lock()
			results[check.Name] = exists
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results
}

// ResourceCheck defines a resource existence check.
type ResourceCheck struct {
	Type string // "pipeline", "recipe", "infra", "dist", "component"
	Name string
}

// checkResourceExists checks if a single resource exists.
func (bo *BatchOperations) checkResourceExists(ctx context.Context, check ResourceCheck) bool {
	resourceManager := NewResourceManager(bo.clients)

	switch check.Type {
	case "pipeline":
		pipeline, err := resourceManager.GetImagePipeline(ctx, check.Name)
		return pipeline != nil && !errors.Is(err, ErrNotFound)
	case "recipe":
		recipe, err := resourceManager.GetImageRecipe(ctx, check.Name, "")
		return recipe != nil && !errors.Is(err, ErrNotFound)
	case "infra":
		infra, err := resourceManager.GetInfrastructureConfig(ctx, check.Name)
		return infra != nil && !errors.Is(err, ErrNotFound)
	case "dist":
		dist, err := resourceManager.GetDistributionConfig(ctx, check.Name)
		return dist != nil && !errors.Is(err, ErrNotFound)
	default:
		return false
	}
}

// OptimizedResourceCleanup performs cleanup with batched operations.
// Deletes resources in dependency order: pipeline -> recipe -> (dist, infra).
func (bo *BatchOperations) OptimizedResourceCleanup(ctx context.Context, buildName string) error {
	logging.InfoContext(ctx, "Running optimized resource cleanup for: %s", buildName)

	// First, check which resources exist (batched)
	checks := []ResourceCheck{
		{Type: "pipeline", Name: buildName + "-pipeline"},
		{Type: "recipe", Name: buildName + "-recipe"},
		{Type: "dist", Name: buildName + "-dist"},
		{Type: "infra", Name: buildName + "-infra"},
	}

	existence := bo.BatchCheckResourceExistence(ctx, checks)

	// Delete in dependency order (pipeline -> recipe -> dist/infra)
	resourceManager := NewResourceManager(bo.clients)

	if existence[buildName+"-pipeline"] {
		if err := resourceManager.DeleteImagePipeline(ctx, buildName+"-pipeline"); err != nil {
			logging.WarnContext(ctx, "Failed to delete pipeline: %v", err)
		}
	}

	if existence[buildName+"-recipe"] {
		if arn, err := bo.getRecipeARN(ctx, buildName+"-recipe"); err == nil && arn != "" {
			if err := resourceManager.DeleteImageRecipe(ctx, arn); err != nil {
				logging.WarnContext(ctx, "Failed to delete recipe: %v", err)
			}
		}
	}

	// Delete dist and infra concurrently since they don't depend on each other
	// Using WaitGroup since we're logging errors rather than propagating them
	var wg sync.WaitGroup

	if existence[buildName+"-dist"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if arn, err := bo.getDistARN(ctx, buildName+"-dist"); err == nil && arn != "" {
				if err := resourceManager.DeleteDistributionConfig(ctx, arn); err != nil {
					logging.WarnContext(ctx, "Failed to delete distribution config: %v", err)
				}
			}
		}()
	}

	if existence[buildName+"-infra"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if arn, err := bo.getInfraARN(ctx, buildName+"-infra"); err == nil && arn != "" {
				if err := resourceManager.DeleteInfrastructureConfig(ctx, arn); err != nil {
					logging.WarnContext(ctx, "Failed to delete infrastructure config: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	logging.InfoContext(ctx, "Optimized cleanup completed for: %s", buildName)
	return nil
}

// getRecipeARN retrieves the ARN of an image recipe by name.
func (bo *BatchOperations) getRecipeARN(ctx context.Context, name string) (string, error) {
	resourceManager := NewResourceManager(bo.clients)
	recipe, err := resourceManager.GetImageRecipe(ctx, name, "")
	if errors.Is(err, ErrNotFound) {
		return "", nil // Not found is not an error for ARN lookup
	}
	if err != nil {
		return "", err
	}
	return aws.ToString(recipe.Arn), nil
}

// getDistARN retrieves the ARN of a distribution configuration by name.
func (bo *BatchOperations) getDistARN(ctx context.Context, name string) (string, error) {
	resourceManager := NewResourceManager(bo.clients)
	dist, err := resourceManager.GetDistributionConfig(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return "", nil // Not found is not an error for ARN lookup
	}
	if err != nil {
		return "", err
	}
	return aws.ToString(dist.Arn), nil
}

// getInfraARN retrieves the ARN of an infrastructure configuration by name.
func (bo *BatchOperations) getInfraARN(ctx context.Context, name string) (string, error) {
	resourceManager := NewResourceManager(bo.clients)
	infra, err := resourceManager.GetInfrastructureConfig(ctx, name)
	if errors.Is(err, ErrNotFound) {
		return "", nil // Not found is not an error for ARN lookup
	}
	if err != nil {
		return "", err
	}
	return aws.ToString(infra.Arn), nil
}
