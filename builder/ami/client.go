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

package ami

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/imagebuilder"
)

// AWSClients holds AWS service clients
type AWSClients struct {
	EC2          *ec2.Client
	ImageBuilder *imagebuilder.Client
	Config       aws.Config
}

// ClientConfig contains configuration for creating AWS clients
type ClientConfig struct {
	Region          string
	Profile         string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// NewAWSClients creates a new set of AWS clients with the given configuration
func NewAWSClients(ctx context.Context, cfg ClientConfig) (*AWSClients, error) {
	var optFns []func(*config.LoadOptions) error

	// Set region if provided
	if cfg.Region != "" {
		optFns = append(optFns, config.WithRegion(cfg.Region))
	}

	// Set profile if provided
	if cfg.Profile != "" {
		optFns = append(optFns, config.WithSharedConfigProfile(cfg.Profile))
	}

	// Set static credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		provider := credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
		optFns = append(optFns, config.WithCredentialsProvider(provider))
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Validate that region is set (either from config or environment)
	if awsCfg.Region == "" {
		return nil, fmt.Errorf("AWS region not specified (set AWS_REGION or pass region in config)")
	}

	// Create service clients
	clients := &AWSClients{
		EC2:          ec2.NewFromConfig(awsCfg),
		ImageBuilder: imagebuilder.NewFromConfig(awsCfg),
		Config:       awsCfg,
	}

	return clients, nil
}

// GetRegion returns the configured AWS region
func (c *AWSClients) GetRegion() string {
	return c.Config.Region
}
