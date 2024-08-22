package s3

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Region          string
	Endpoint        string
	AccessKeyId     string
	SecretAccessKey string
}

func Connect() {
	cfgStruct := Config{
		Region:          "us-east-1",
		Endpoint:        "http://localstack:4566",
		AccessKeyId:     "test",
		SecretAccessKey: "test",
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfgStruct.Region),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     cfgStruct.AccessKeyId,
				SecretAccessKey: cfgStruct.SecretAccessKey,
			},
		}),
	)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfgStruct.Endpoint)
	})

	// Create a bucket (if it doesn't exist)
	_, err = client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		if err.Error() != "BucketAlreadyOwnedByYou" {
			log.Fatalf("Failed to create bucket: %v", err)
		}
	}

	// Create a file in the bucket
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String("my-bucket"),
		Key:         aws.String("my-file.txt"),
		Body:        strings.NewReader("Hello, world!"),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}

	// List objects in the bucket
	result, err := client.ListObjects(context.TODO(), &s3.ListObjectsInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		log.Fatalf("Failed to list objects: %v", err)
	}

	// Print the list of objects
	for _, obj := range result.Contents {
		fmt.Println(*obj.Key)
	}
}
