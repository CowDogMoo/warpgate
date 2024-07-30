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
// **Parameters:**
//
// cs: CloudStorage configuration.
//
// **Returns:**
//
// string: A unique bucket name.
func createBucketName(cs *CloudStorage) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s-bucket-%d", cs.BlueprintName, timestamp)
}

// CreateS3Bucket initializes an S3 bucket and stores the bucket name.
//
// **Parameters:**
//
// cs: CloudStorage configuration.
//
// **Returns:**
//
// error: An error if the S3 bucket initialization fails.
func CreateS3Bucket(cs *CloudStorage) error {
	if cs.BucketName == "" {
		cs.BucketName = createBucketName(cs)
		conn := s3.CreateConnection()
		err := s3.CreateBucket(conn.Client, cs.BucketName)
		if err != nil {
			return fmt.Errorf("failed to create S3 bucket: %v", err)
		}
		fmt.Printf("Created S3 bucket: %s\n", cs.BucketName)
	} else {
		fmt.Printf("Using existing S3 bucket: %s\n", cs.BucketName)
	}
	return nil
}

// DestroyS3Bucket destroys the S3 bucket created for the blueprint.
//
// **Parameters:**
//
// cs: CloudStorage configuration.
//
// **Returns:**
//
// error: An error if the S3 bucket destruction fails.
func DestroyS3Bucket(cs *CloudStorage) error {
	conn := s3.CreateConnection()
	err := s3.DestroyBucket(conn.Client, cs.BucketName)
	if err != nil {
		return fmt.Errorf("failed to destroy S3 bucket: %v", err)
	}

	fmt.Printf("Destroyed S3 bucket: %s\n", cs.BucketName)
	return nil
}
