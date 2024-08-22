package s3

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func Connect() error {

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	localStackEndpoint := "http://localstack:4566"
	s3Client := s3.NewFromConfig(
		cfg,
		func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = &localStackEndpoint
		},
	)
	fmt.Println("s3 client created")

	// Create Bucket
	_, err = s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String("my-bucket"),
	})
	if err != nil {
		return err
	}
	fmt.Println("s3 bucket created")

	//count := 10
	//fmt.Printf("Let's list up to %v buckets for your account.\n", count)
	//result, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	//if err != nil {
	//	fmt.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
	//	return err
	//}
	//if len(result.Buckets) == 0 {
	//	fmt.Println("You don't have any buckets!")
	//} else {
	//	if count > len(result.Buckets) {
	//		count = len(result.Buckets)
	//	}
	//	for _, bucket := range result.Buckets[:count] {
	//		fmt.Printf("\t%v\n", *bucket.Name)
	//	}
	//}
	return nil
}
