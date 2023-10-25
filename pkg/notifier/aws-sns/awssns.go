/*
Package awssns provides notifier implementation for AWS Simple Notification Service(SNS) to publish-subscribe messages
to an SNS topic.It offers features like message attribute customization and health checks to ensure the notifier's availability.
*/
package awssns

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/notifier"
)

// SNS is a struct representing an AWS SNS notifier and its configuration.
type SNS struct {
	sns AWS
	cfg *Config
}

// Config represents the configuration for an AWS SNS notifier.
type Config struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	// ConnRetryDuration for specifying connection retry duration
	ConnRetryDuration int
	TopicArn          string
	Endpoint          string
	Protocol          string
}

//nolint // The declared global variable can be accessed across multiple functions
var (
	notifierReceiveCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "zs_notifier_receive_count",
		Help: "Total number of subscribe operation",
	}, []string{"topic"})

	notifierSuccessCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "zs_notifier_success_count",
		Help: "Total number of successful subscribe operation",
	}, []string{"topic"})

	notifierFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "zs_notifier_failure_count",
		Help: "Total number of failed subscribe operation",
	}, []string{"topic"})

	_ = prometheus.Register(notifierReceiveCount)
	_ = prometheus.Register(notifierSuccessCount)
	_ = prometheus.Register(notifierFailureCount)
)

// New is a factory function creates and configures an SNS
func New(c *Config) (notifier.Notifier, error) {
	sessionConfig := &aws.Config{
		Region:      aws.String(c.Region),
		Credentials: credentials.NewStaticCredentials(c.AccessKeyID, c.SecretAccessKey, ""),
	}

	if c.Endpoint != "" {
		sessionConfig.Endpoint = &c.Endpoint
	}

	sess, err := session.NewSession(sessionConfig)
	if err != nil {
		return nil, err
	}

	svc := sns.New(sess, nil)

	return &SNS{sns: svc, cfg: c}, nil
}

// Publish a message to an AWS SNS topic, handling value marshaling and message attribute construction
func (s *SNS) Publish(value interface{}, attributes map[string]interface{}) (err error) {
	data, ok := value.([]byte)
	if !ok {
		data, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}

	input := &sns.PublishInput{
		Message:           aws.String(string(data)),
		TopicArn:          aws.String(s.cfg.TopicArn),
		MessageAttributes: getMessageAttributes(attributes),
	}

	_, err = s.sns.Publish(input)
	if err != nil {
		return err
	}

	return nil
}

// Subscribe  to an AWS SNS topic, increments metrics based on success or failure.
// It returns a message with subscription details.
func (s *SNS) Subscribe() (*notifier.Message, error) {
	// for every subscribe
	notifierReceiveCount.WithLabelValues(s.cfg.Endpoint).Inc()

	out, err := s.sns.Subscribe(&sns.SubscribeInput{
		Endpoint:              &s.cfg.Endpoint,
		Protocol:              &s.cfg.Protocol,
		ReturnSubscriptionArn: aws.Bool(true), // Return the ARN, even if user has yet to confirm
		TopicArn:              &s.cfg.TopicArn,
	})

	if err != nil {
		// for failed subscribe
		notifierFailureCount.WithLabelValues(s.cfg.Endpoint).Inc()
		return nil, err
	}

	msg, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	// for successful subscribe
	notifierSuccessCount.WithLabelValues(s.cfg.Endpoint).Inc()

	return &notifier.Message{Value: string(msg)}, nil
}

// SubscribeWithResponse subscribes to an AWS SNS topic
func (s *SNS) SubscribeWithResponse(target interface{}) (*notifier.Message, error) {
	message, err := s.Subscribe()
	if err != nil {
		return message, err
	}

	return message, s.Bind([]byte(message.Value), &target)
}

// Bind unmarshals a JSON message into a target interface,
func (s *SNS) Bind(message []byte, target interface{}) error {
	return json.Unmarshal(message, target)
}

func (s *SNS) ping() error {
	sessionConfig := &aws.Config{
		Region:      aws.String(s.cfg.Region),
		Credentials: credentials.NewStaticCredentials(s.cfg.AccessKeyID, s.cfg.SecretAccessKey, ""),
	}

	sess, _ := session.NewSession(sessionConfig)

	svc := sns.New(sess, nil)
	if svc == nil {
		return errors.AWSSessionNotCreated
	}

	return nil
}

// HealthCheck reports the health status of an AWS SNS client, indicating whether it is up or down
func (s *SNS) HealthCheck() types.Health {
	if s == nil {
		return types.Health{
			Name:   datastore.AWSSNS,
			Status: pkg.StatusDown,
		}
	}

	resp := types.Health{
		Name:   datastore.AWSSNS,
		Status: pkg.StatusDown,
		Host:   s.cfg.TopicArn,
	}

	if err := s.ping(); err != nil {
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// IsSet checks if the SNS client is set and non-nil.
func (s *SNS) IsSet() bool {
	if s == nil {
		return false
	}

	return s.sns != nil
}

func getMessageAttributes(mp map[string]interface{}) map[string]*sns.MessageAttributeValue {
	if mp == nil {
		return nil
	}

	values := make(map[string]*sns.MessageAttributeValue)

	for key, val := range mp {
		av := &sns.MessageAttributeValue{}

		var dataType, value string

		switch val.(type) {
		case int, int64, float64:
			dataType = "Number"
			value = fmt.Sprintf("%v", val)
		case []int64, []float64, []string, []interface{}:
			dataType = "String.Array"

			data, _ := json.Marshal(val)

			value = string(data)
		default:
			dataType = "String"
			value = fmt.Sprintf("%v", val)
		}

		av.SetDataType(dataType)
		av.SetStringValue(value)

		values[key] = av
	}

	return values
}
