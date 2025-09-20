package dynamodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	errClientNotConnected = errors.New("client not connected, call Connect() first")
	errKeyNotFound        = errors.New("key not found")
	errStatusDown         = errors.New("status down")
)

type Configs struct {
	Table            string
	Region           string
	Endpoint         string
	PartitionKeyName string
}
type dynamoDBInterface interface {
	PutItem(
		ctx context.Context,
		params *dynamodb.PutItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.PutItemOutput, error)
	GetItem(
		ctx context.Context,
		params *dynamodb.GetItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.GetItemOutput, error)
	DeleteItem(
		ctx context.Context,
		params *dynamodb.DeleteItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.DeleteItemOutput, error)
	DescribeTable(
		ctx context.Context,
		params *dynamodb.DescribeTableInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.DescribeTableOutput, error)
}

type Client struct {
	db        dynamoDBInterface
	configs   *Configs
	logger    Logger
	metrics   Metrics
	tracer    trace.Tracer
	connected bool
}

func New(configs Configs) *Client {
	if configs.PartitionKeyName == "" {
		configs.PartitionKeyName = "pk"
	}

	return &Client{configs: &configs, connected: false}
}

// Connect establishes a connection to DynamoDB and registers metrics using the provided configuration.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to DynamoDB table %v in region %v", c.configs.Table, c.configs.Region)

	dynamoBuckets := []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000}
	c.metrics.NewHistogram("app_dynamodb_duration_ms", "Response time of DynamoDB queries in milliseconds.", dynamoBuckets...)

	awsCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(c.configs.Region))
	if err != nil {
		c.logger.Errorf("error loading AWS config: %v", err)
		return
	}

	var opts []func(*dynamodb.Options)

	if c.configs.Endpoint != "" {
		opts = append(opts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(c.configs.Endpoint)
		})
	}

	db := dynamodb.NewFromConfig(awsCfg, opts...)
	c.db = db
	c.connected = true

	c.logger.Infof("connected to DynamoDB table %v in region %v", c.configs.Table, c.configs.Region)
}

// UseLogger sets the logger for the Dynamo client which asserts the Logger interface.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Dynamo client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for Dynamo client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// ToJSON converts a Go struct to JSON string for use with KVStore.Set.
func ToJSON(value any) (string, error) {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal value to JSON: %w", err)
	}

	return string(jsonData), nil
}

// FromJSON converts a JSON string to a Go struct for use with KVStore.Get.
func FromJSON(jsonData string, dest any) error {
	if err := json.Unmarshal([]byte(jsonData), dest); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if !c.connected {
		return "", errClientNotConnected
	}

	span := c.addTrace(ctx, "get", key)
	defer c.sendOperationsStats(time.Now(), "GET", "get", span, key)

	input := &dynamodb.GetItemInput{
		TableName: aws.String(c.configs.Table),
		Key: map[string]types.AttributeValue{
			c.configs.PartitionKeyName: &types.AttributeValueMemberS{Value: key},
		},
	}

	out, err := c.db.GetItem(ctx, input)
	if err != nil {
		c.logger.Errorf("error while fetching data for key: %v, error: %v", key, err)
		return "", err
	}

	if out.Item == nil {
		return "", errKeyNotFound
	}

	// Look for a "value" field that contains the JSON string
	if valueField, exists := out.Item["value"]; exists {
		if stringValue, ok := valueField.(*types.AttributeValueMemberS); ok {
			return stringValue.Value, nil
		}
	}

	// If no "value" field exists, return key not found error
	return "", fmt.Errorf("%w: %s", errKeyNotFound, key)
}

func (c *Client) Set(ctx context.Context, key, value string) error {
	if !c.connected {
		return errClientNotConnected
	}

	span := c.addTrace(ctx, "set", key)
	defer c.sendOperationsStats(time.Now(), "SET", "set", span, key)

	// Store the value as a string in the "value" field
	item := map[string]types.AttributeValue{
		c.configs.PartitionKeyName: &types.AttributeValueMemberS{Value: key},
		"value":                    &types.AttributeValueMemberS{Value: value},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(c.configs.Table),
		Item:      item,
	}

	_, err := c.db.PutItem(ctx, input)
	if err != nil {
		c.logger.Errorf("error while setting data for key: %v, error: %v", key, err)
		return err
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if !c.connected {
		return errClientNotConnected
	}

	span := c.addTrace(ctx, "delete", key)
	defer c.sendOperationsStats(time.Now(), "DELETE", "delete", span, key)

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(c.configs.Table),
		Key: map[string]types.AttributeValue{
			c.configs.PartitionKeyName: &types.AttributeValueMemberS{Value: key},
		},
	}

	_, err := c.db.DeleteItem(ctx, input)

	if err != nil {
		c.logger.Errorf("error while deleting data for key: %v, error: %v", key, err)

		return err
	}

	return nil
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	if !c.connected {
		return &Health{Status: "DOWN", Details: map[string]any{"error": "client not connected"}}, errStatusDown
	}

	h := Health{
		Details: make(map[string]any),
	}

	h.Details["table"] = c.configs.Table
	h.Details["region"] = c.configs.Region

	input := &dynamodb.DescribeTableInput{TableName: aws.String(c.configs.Table)}

	_, err := c.db.DescribeTable(ctx, input)
	if err != nil {
		h.Status = "DOWN"

		return &h, errStatusDown
	}

	h.Status = "UP"

	return &h, nil
}

func (c *Client) sendOperationsStats(start time.Time, methodType, method string, span trace.Span, kv ...string) {
	duration := time.Since(start)

	var key string
	if len(kv) > 0 {
		key = kv[0]
	}

	c.logger.Debug(&Log{
		Type:     methodType,
		Duration: duration.Microseconds(),
		Key:      key,
		Value:    c.configs.Table,
	})

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("dynamodb.%v.duration(μs)", method), duration.Microseconds()))
	}

	c.metrics.RecordHistogram(context.Background(), "app_dynamodb_duration_ms", float64(duration.Milliseconds()),
		"table", c.configs.Table,
		"operation", methodType)
}

func (c *Client) addTrace(ctx context.Context, method, key string) trace.Span {
	if c.tracer != nil {
		_, span := c.tracer.Start(ctx, fmt.Sprintf("dynamodb-%v", method))
		span.SetAttributes(
			attribute.String("dynamodb.method", method),
			attribute.String("dynamodb.key", key),
		)

		return span
	}

	return nil
}
