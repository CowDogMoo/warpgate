package cloudstorage

import (
	"fmt"
	"time"

	"github.com/l50/awsutils/s3"
)

// CloudStorage represents the configuration needed for S3 bucket operations.
//
// **Attributes:**
//
// BlueprintName: Name of the blueprint.
// BucketName: Dynamically created bucket name.
type CloudStorage struct {
	BlueprintName string
	BucketName    string
}

// createBucketName generates a unique bucket name based on the blueprint name and timestamp.
//
// **Returns:**
//
// string: A unique bucket name.
func createBucketName(cs *CloudStorage) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-bucket-%d", cs.BlueprintName, timestamp)
}

// CleanupBucket destroys the S3 bucket created for the blueprint.
//
// **Returns:**
//
// error: An error if the S3 bucket cleanup fails.
func CleanupBucket(cs *CloudStorage) error {
	conn := s3.CreateConnection()
	if err := s3.DestroyBucket(conn.Client, cs.BucketName); err != nil {
		return fmt.Errorf("failed to destroy S3 bucket: %v", err)
	}

	fmt.Printf("Destroyed S3 bucket: %s\n", cs.BucketName)
	return nil
}

// InitializeS3Bucket initializes an S3 bucket and stores the bucket name.
//
// **Returns:**
//
// error: An error if the S3 bucket initialization fails.
func InitializeS3Bucket(cs *CloudStorage) error {
	conn := s3.CreateConnection()
	bucketName := createBucketName(cs)

	err := s3.CreateBucket(conn.Client, bucketName)
	if err != nil {
		return fmt.Errorf("failed to create S3 bucket: %v", err)
	}

	cs.BucketName = bucketName
	fmt.Printf("Created S3 bucket: %s\n", bucketName)
	return nil
}
