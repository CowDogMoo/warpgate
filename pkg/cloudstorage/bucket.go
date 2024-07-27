package cloudstorage

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// CloudStorage represents the configuration needed for S3 bucket operations.
//
// **Attributes:**
//
// BlueprintName: Name of the blueprint.
// BucketName: Dynamically created bucket name.
// Client: AWS S3 client.
type CloudStorage struct {
	BlueprintName string
	BucketName    string
	Client        s3iface.S3API
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

// createBucket creates an S3 bucket.
//
// **Parameters:**
//
// client: AWS S3 client.
// bucketName: Name of the bucket to be created.
//
// **Returns:**
//
// error: An error if the S3 bucket creation fails.
func createBucket(client s3iface.S3API, bucketName string) error {
	_, err := client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	return err
}

// destroyBucket destroys an S3 bucket.
//
// **Parameters:**
//
// client: AWS S3 client.
// bucketName: Name of the bucket to be destroyed.
//
// **Returns:**
//
// error: An error if the S3 bucket destruction fails.
func destroyBucket(client s3iface.S3API, bucketName string) error {
	_, err := client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		waitErr := client.WaitUntilBucketNotExists(&s3.HeadBucketInput{Bucket: aws.String(bucketName)})
		if waitErr != nil {
			return waitErr
		}
		return err
	}

	err = client.WaitUntilBucketNotExists(&s3.HeadBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		return err
	}

	return nil
}

// CreateS3Bucket initializes an S3 bucket and stores the bucket name.
//
// **Returns:**
//
// error: An error if the S3 bucket initialization fails.
func CreateS3Bucket(cs *CloudStorage) error {
	bucketName := createBucketName(cs)

	err := createBucket(cs.Client, bucketName)
	if err != nil {
		return fmt.Errorf("failed to create S3 bucket: %v", err)
	}

	cs.BucketName = bucketName
	fmt.Printf("Created S3 bucket: %s\n", bucketName)
	return nil
}

// DestroyS3Bucket destroys the S3 bucket created for the blueprint.
//
// **Returns:**
//
// error: An error if the S3 bucket destruction fails.
func DestroyS3Bucket(cs *CloudStorage) error {
	err := destroyBucket(cs.Client, cs.BucketName)
	if err != nil {
		return fmt.Errorf("failed to destroy S3 bucket: %v", err)
	}

	fmt.Printf("Destroyed S3 bucket: %s\n", cs.BucketName)
	return nil
}
