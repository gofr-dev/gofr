package s3

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func Connect() error {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("LKIAQAAAAAAAF6MXEBEH", "+Kwf4m/IHqglJ+LfmEx9aGZQJs4GdC94V8TJKHiV", "")),
	)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create the S3 client with the custom endpoint
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
		o.UsePathStyle = true
	})

	fmt.Println("s3 client created")

	// Create Bucket
	_, err = s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	fmt.Println("s3 bucket created")

	// List Buckets
	result, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	fmt.Println("Buckets:")
	for _, bucket := range result.Buckets {
		fmt.Printf(" - %s\n", *bucket.Name)
	}

	// Put Keys
	s3Key1 := "key1"
	body1 := []byte(fmt.Sprintf("Hello from localstack 1"))
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:             aws.String("my-bucket"),
		Key:                aws.String(s3Key1),
		Body:               bytes.NewReader(body1),
		ContentType:        aws.String("application/text"),
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	s3Key2 := "key2"
	body2 := []byte(fmt.Sprintf("Hello from localstack 2. This is Golang."))
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:             aws.String("my-bucket"),
		Key:                aws.String(s3Key2),
		Body:               bytes.NewReader(body2),
		ContentType:        aws.String("application/text"),
		ContentDisposition: aws.String("attachment"),
	})
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	// List Objects
	output, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("List of Objects in %s:\n", "my-bucket")
	for _, object := range output.Contents {
		fmt.Printf("key=%s size=%d\n", aws.ToString(object.Key), object.Size)
	}

	return nil
}
