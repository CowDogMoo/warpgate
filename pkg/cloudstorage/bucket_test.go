package cloudstorage_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cowdogmoo/warpgate/pkg/cloudstorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockS3Client is a mock implementation of the S3API interface.
type MockS3Client struct {
	mock.Mock
	s3.S3
}

func (m *MockS3Client) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.CreateBucketOutput), args.Error(1)
}

func (m *MockS3Client) DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.DeleteBucketOutput), args.Error(1)
}

func (m *MockS3Client) WaitUntilBucketNotExists(input *s3.HeadBucketInput) error {
	args := m.Called(input)
	return args.Error(0)
}

func (m *MockS3Client) ListBuckets(input *s3.ListBucketsInput) (*s3.ListBucketsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3.ListBucketsOutput), args.Error(1)
}

func TestCreateBucketName(t *testing.T) {
	tests := []struct {
		name          string
		blueprintName string
		expected      string
	}{
		{
			name:          "valid blueprint name",
			blueprintName: "test-blueprint",
			expected:      "test-blueprint-bucket-",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockS3 := &MockS3Client{}
			mockS3.On("CreateBucket", mock.AnythingOfType("*s3.CreateBucketInput")).Return(&s3.CreateBucketOutput{}, nil)

			cs := &cloudstorage.CloudStorage{
				BlueprintName: tc.blueprintName,
				Client:        mockS3,
			}

			err := cloudstorage.InitializeS3Bucket(cs)
			assert.NoError(t, err)
			assert.Contains(t, cs.BucketName, tc.expected)

			mockS3.AssertExpectations(t)
		})
	}
}

func TestCleanupBucket(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		setup      func(*MockS3Client)
		expectErr  bool
	}{
		{
			name:       "successful cleanup",
			bucketName: "test-bucket-cleanup",
			setup: func(mockS3 *MockS3Client) {
				mockS3.On("DeleteBucket", mock.AnythingOfType("*s3.DeleteBucketInput")).Return(&s3.DeleteBucketOutput{}, nil)
				mockS3.On("WaitUntilBucketNotExists", mock.AnythingOfType("*s3.HeadBucketInput")).Return(nil)
			},
			expectErr: false,
		},
		{
			name:       "failed cleanup",
			bucketName: "non-existent-bucket",
			setup: func(mockS3 *MockS3Client) {
				mockS3.On("DeleteBucket", mock.AnythingOfType("*s3.DeleteBucketInput")).Return(nil, assert.AnError)
				mockS3.On("WaitUntilBucketNotExists", mock.AnythingOfType("*s3.HeadBucketInput")).Return(assert.AnError)
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockS3 := &MockS3Client{}
			tc.setup(mockS3)

			cs := &cloudstorage.CloudStorage{
				BucketName: tc.bucketName,
				Client:     mockS3,
			}

			err := cloudstorage.CleanupBucket(cs)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockS3.AssertExpectations(t)
		})
	}
}

func TestInitializeS3Bucket(t *testing.T) {
	tests := []struct {
		name          string
		blueprintName string
		setup         func(*MockS3Client)
		expectErr     bool
	}{
		{
			name:          "successful initialization",
			blueprintName: "test-blueprint",
			setup: func(mockS3 *MockS3Client) {
				mockS3.On("CreateBucket", mock.AnythingOfType("*s3.CreateBucketInput")).Return(&s3.CreateBucketOutput{}, nil)
			},
			expectErr: false,
		},
		{
			name:          "failed initialization",
			blueprintName: "",
			setup: func(mockS3 *MockS3Client) {
				mockS3.On("CreateBucket", mock.AnythingOfType("*s3.CreateBucketInput")).Return(nil, assert.AnError)
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockS3 := &MockS3Client{}
			tc.setup(mockS3)

			cs := &cloudstorage.CloudStorage{
				BlueprintName: tc.blueprintName,
				Client:        mockS3,
			}

			err := cloudstorage.InitializeS3Bucket(cs)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, cs.BucketName, "test-blueprint-bucket-")
			}

			mockS3.AssertExpectations(t)
		})
	}
}
