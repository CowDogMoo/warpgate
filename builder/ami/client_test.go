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

package ami

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAWSClients_WithRegion(t *testing.T) {
	old := loadAWSConfig
	defer func() { loadAWSConfig = old }()

	loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{Region: "us-west-2"}, nil
	}

	clients, err := NewAWSClients(context.Background(), ClientConfig{Region: "us-west-2"})
	require.NoError(t, err)
	assert.Equal(t, "us-west-2", clients.GetRegion())
	assert.NotNil(t, clients.EC2)
	assert.NotNil(t, clients.ImageBuilder)
	assert.NotNil(t, clients.IAM)
	assert.NotNil(t, clients.SSM)
	assert.NotNil(t, clients.CloudWatchLogs)
}

func TestNewAWSClients_WithProfile(t *testing.T) {
	old := loadAWSConfig
	defer func() { loadAWSConfig = old }()

	var capturedOpts config.LoadOptions
	loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		for _, fn := range optFns {
			if err := fn(&capturedOpts); err != nil {
				return aws.Config{}, err
			}
		}
		return aws.Config{Region: "us-east-1"}, nil
	}

	clients, err := NewAWSClients(context.Background(), ClientConfig{
		Region:  "us-east-1",
		Profile: "my-profile",
	})
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", clients.GetRegion())
	assert.Equal(t, "my-profile", capturedOpts.SharedConfigProfile)
}

func TestNewAWSClients_WithStaticCredentials(t *testing.T) {
	old := loadAWSConfig
	defer func() { loadAWSConfig = old }()

	var capturedOpts config.LoadOptions
	loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		for _, fn := range optFns {
			if err := fn(&capturedOpts); err != nil {
				return aws.Config{}, err
			}
		}
		return aws.Config{Region: "eu-west-1"}, nil
	}

	clients, err := NewAWSClients(context.Background(), ClientConfig{
		Region:          "eu-west-1",
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secretkey123",
		SessionToken:    "sessiontoken456",
	})
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", clients.GetRegion())
	// Verify that credentials provider was set (non-nil)
	assert.NotNil(t, capturedOpts.Credentials)
}

func TestNewAWSClients_EmptyRegionError(t *testing.T) {
	old := loadAWSConfig
	defer func() { loadAWSConfig = old }()

	loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{Region: ""}, nil
	}

	clients, err := NewAWSClients(context.Background(), ClientConfig{})
	assert.Error(t, err)
	assert.Nil(t, clients)
	assert.Contains(t, err.Error(), "AWS region not specified")
}

func TestNewAWSClients_LoadError(t *testing.T) {
	old := loadAWSConfig
	defer func() { loadAWSConfig = old }()

	loadAWSConfig = func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, fmt.Errorf("config load failed: network error")
	}

	clients, err := NewAWSClients(context.Background(), ClientConfig{Region: "us-east-1"})
	assert.Error(t, err)
	assert.Nil(t, clients)
	assert.Contains(t, err.Error(), "failed to load AWS config")
	assert.Contains(t, err.Error(), "network error")
}

func TestGetRegion(t *testing.T) {
	t.Parallel()

	clients := &AWSClients{
		Config: aws.Config{Region: "ap-southeast-1"},
	}

	assert.Equal(t, "ap-southeast-1", clients.GetRegion())
}
