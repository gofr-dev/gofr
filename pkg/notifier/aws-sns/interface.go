package awssns

import "github.com/aws/aws-sdk-go/service/sns"

// AWS is an interface for interacting with the AWS Simple Notification Service (SNS).
type AWS interface {
	Subscribe(input *sns.SubscribeInput) (*sns.SubscribeOutput, error)
	Publish(input *sns.PublishInput) (*sns.PublishOutput, error)
}
