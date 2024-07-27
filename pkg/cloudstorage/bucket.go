package cloudstorage

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	s3utils "github.com/l50/awsutils/s3"
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

// CreateBucketWrapper is a wrapper function for creating an S3 bucket.
//
// **Parameters:**
//
// client: AWS S3 client.
// bucketName: Name of the bucket to be created.
//
// **Returns:**
//
// error: An error if the S3 bucket creation fails.
func CreateBucketWrapper(client s3iface.S3API, bucketName string) error {
	s3Client, ok := client.(*s3.S3)
	if !ok {
		return fmt.Errorf("invalid S3 client type")
	}
	return s3utils.CreateBucket(s3Client, bucketName)
}

// CleanupBucket destroys the S3 bucket created for the blueprint.
//
// **Returns:**
//
// error: An error if the S3 bucket cleanup fails.
func CleanupBucket(cs *CloudStorage) error {
	err := DestroyBucketAPI(cs.Client, cs.BucketName)
	if err != nil {
		return fmt.Errorf("failed to destroy S3 bucket: %v", err)
	}

	fmt.Printf("Destroyed S3 bucket: %s\n", cs.BucketName)
	return nil
}

func CreateBucketAPI(client s3iface.S3API, bucketName string) error {
	_, err := client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	return err
}

func DestroyBucketAPI(client s3iface.S3API, bucketName string) error {
	_, err := client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		return err
	}

	err = client.WaitUntilBucketNotExists(&s3.HeadBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		return err
	}

	return nil
}

// InitializeS3Bucket initializes an S3 bucket and stores the bucket name.
//
// **Returns:**
//
// error: An error if the S3 bucket initialization fails.
func InitializeS3Bucket(cs *CloudStorage) error {
	bucketName := createBucketName(cs)

	err := CreateBucketAPI(cs.Client, bucketName)
	if err != nil {
		return fmt.Errorf("failed to create S3 bucket: %v", err)
	}

	cs.BucketName = bucketName
	fmt.Printf("Created S3 bucket: %s\n", bucketName)
	return nil
}
