package datastore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/prometheus/client_golang/prometheus"

	"gofr.dev/pkg"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

//nolint:gochecknoglobals // dynamodbStats has to be a global variable for prometheus
var (
	dynamodbStats = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "zs_dynamodb_stats",
		Help:    "Histogram for DynamoDB",
		Buckets: []float64{.001, .003, .005, .01, .025, .05, .1, .2, .3, .4, .5, .75, 1, 2, 3, 5, 10, 30},
	}, []string{"type", "host", "table"})

	_ = prometheus.Register(dynamodbStats)
)

// DynamoDBConfig configuration for DynamoDB connection
type DynamoDBConfig struct {
	Region            string
	Endpoint          string
	AccessKeyID       string
	SecretAccessKey   string
	ConnRetryDuration int
}

// DynamoDB stores the DynamoDB Client along with logger ad configs to connect to DynamoDB Database.
type DynamoDB struct {
	*dynamodb.DynamoDB
	logger log.Logger
	config DynamoDBConfig
}

// NewDynamoDB connects to DynamoDB and returns the connection
func NewDynamoDB(logger log.Logger, c DynamoDBConfig) (DynamoDB, error) {
	sessionConfig := &aws.Config{
		Region:      aws.String(c.Region),
		Logger:      logger,
		Endpoint:    aws.String(c.Endpoint),
		Credentials: credentials.NewStaticCredentials(c.AccessKeyID, c.SecretAccessKey, ""),
	}

	var db = DynamoDB{logger: logger, config: c}

	s, err := session.NewSession(sessionConfig)
	if err != nil {
		return db, err
	}

	db.DynamoDB = dynamodb.New(s)

	// we are using a NoOpRetryer to fail faster and make sure that the sdk does not do a retry.
	// Retry is done by gofr implicitly if the connection is not made.
	// If the connection is not made, default retryer be will be used. Refer: github.com/aws/aws-sdk-go/aws/client/default_retryer.go
	// this retryer can be used for all the operations of dynamo or can be overwritten.
	db.Retryer = client.NoOpRetryer{}
	defer func() { // resetting the retryer back to default
		db.Retryer = client.DefaultRetryer{}
	}()

	// check the db connection, limit the list to 1 to avoid retrieving large data set
	_, err = db.ListTables(&dynamodb.ListTablesInput{Limit: aws.Int64(1)})
	if err != nil {
		return db, err
	}

	return db, nil
}

// HealthCheck checks health of the Dya
func (d *DynamoDB) HealthCheck() types.Health {
	resp := types.Health{
		Name:   DynamoStore,
		Status: pkg.StatusDown,
	}

	// check if DynamoDB instance has been created during initialization
	if d.DynamoDB == nil {
		d.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: DynamoStore, Reason: "DynamoDB not initialized."})
		return resp
	}

	_, err := d.ListTables(&dynamodb.ListTablesInput{})
	if err != nil {
		d.logger.Errorf("%v", errors.HealthCheckFailed{Dependency: DynamoStore, Err: err})
		return resp
	}

	resp.Status = pkg.StatusUp

	return resp
}

// PutItem adds an item to DynamoDB and monitors the query duration. It records start time, executes the operation, and logs query details.
func (d *DynamoDB) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.PutItem(input)
	duration := time.Since(begin)
	query := genPutItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// PutItemRequest generates a request representing the client's request for the PutItem operation in DynamoDB. It also generates
// a query logger to monitor the query. Returns the request and DynamoDB output which will be populated once the request
// is completed successfully for PutItem.
func (d *DynamoDB) PutItemRequest(input *dynamodb.PutItemInput) (*Request, *dynamodb.PutItemOutput) {
	req, out := d.DynamoDB.PutItemRequest(input)
	query := genPutItemQuery(input)

	ql := genQueryLogger(query)

	ql.Hosts = d.Endpoint
	ql.Logger = d.logger

	return &Request{queryLogger: ql, Request: req}, out
}

// PutItemWithContext adds an item to DynamoDB and monitors the query duration. It records start time, executes the
// operation, and logs query details with the addition of the ability to pass a context and additional request options.
func (d *DynamoDB) PutItemWithContext(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.PutItemWithContext(ctx, input)
	duration := time.Since(begin)
	query := genPutItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// GetItem returns a set of attributes for the item with the given primary key and monitors query duration
// It returns the DynamoDB item and error if any.
func (d *DynamoDB) GetItem(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	begin := time.Now()

	out, err := d.DynamoDB.GetItem(input)

	duration := time.Since(begin)
	query := genGetItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// GetItemRequest generates a request representing the client's request for the GetItem operation in DynamoDB. It also generates
// a query logger to monitor the query. Returns the request and DynamoDB output which will be populated once the request
// is completed successfully for GetItem.
func (d *DynamoDB) GetItemRequest(input *dynamodb.GetItemInput) (*Request, *dynamodb.GetItemOutput) {
	req, out := d.DynamoDB.GetItemRequest(input)
	query := genGetItemQuery(input)
	ql := genQueryLogger(query)

	ql.Hosts = d.Endpoint
	ql.Logger = d.logger

	return &Request{queryLogger: ql, Request: req}, out
}

// GetItemWithContext returns a set of attributes for the item with the given primary key and monitors query duration with the addition
// of the ability to pass a context and additional request options. It returns the DynamoDB item and error if any.
func (d *DynamoDB) GetItemWithContext(ctx context.Context, input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.GetItemWithContext(ctx, input)
	duration := time.Since(begin)
	query := genGetItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// DeleteItem Deletes a single item in a table by primary key.It records start time, executes the operation, and logs query details.
// It returns the output of deleted item and error if any.
func (d *DynamoDB) DeleteItem(input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.DeleteItem(input)
	duration := time.Since(begin)
	query := genDeleteItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// DeleteItemRequest generates a request representing the client's request for the DeleteItem operation.
// It returns the request and Delete Item output which will be populated once the request is completed successfully.
func (d *DynamoDB) DeleteItemRequest(input *dynamodb.DeleteItemInput) (*Request, *dynamodb.DeleteItemOutput) {
	req, out := d.DynamoDB.DeleteItemRequest(input)
	query := genDeleteItemQuery(input)
	ql := genQueryLogger(query)

	ql.Hosts = d.Endpoint
	ql.Logger = d.logger

	return &Request{queryLogger: ql, Request: req}, out
}

/*
DeleteItemWithContext Deletes a single item in a table by primary key with the addition of the ability to pass a context
and additional request options.It records start time, executes the operation, and logs query details.
*/
func (d *DynamoDB) DeleteItemWithContext(ctx context.Context, input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.DeleteItemWithContext(ctx, input)
	duration := time.Since(begin)
	query := genDeleteItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// UpdateItem Edits an existing item's attributes, or adds a new item to the table if it does not already exist.
// It records start time, executes the operation, and logs query details. It returns Update Item output and error if any.
func (d *DynamoDB) UpdateItem(input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.UpdateItem(input)
	duration := time.Since(begin)
	query := genUpdateItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

// UpdateItemRequest generates a request representing the client's request for the UpdateItem operation.
// It also generates a query logger to monitor the query. Returns update request and Update Item output value will be
// populated with the request's response once the request completes successfully.
func (d *DynamoDB) UpdateItemRequest(input *dynamodb.UpdateItemInput) (*Request, *dynamodb.UpdateItemOutput) {
	req, out := d.DynamoDB.UpdateItemRequest(input)
	query := genUpdateItemQuery(input)
	ql := genQueryLogger(query)

	ql.Hosts = d.Endpoint
	ql.Logger = d.logger

	return &Request{queryLogger: ql, Request: req}, out
}

// UpdateItemWithContext Edits an existing item's attributes, or adds a new item to the table if it does not already exist
// with the addition of the ability to pass a context and additional request options. It also generates a query logger to
// monitor the query. Returns update request and Update Item output value will be populated with the request's response
// once the request completes successfully.
func (d *DynamoDB) UpdateItemWithContext(ctx context.Context, input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	begin := time.Now()
	out, err := d.DynamoDB.UpdateItemWithContext(ctx, input)
	duration := time.Since(begin)
	query := genUpdateItemQuery(input)

	d.monitorQuery(query, begin, duration)

	return out, err
}

func (d *DynamoDB) monitorQuery(query []string, begin time.Time, duration time.Duration) {
	table := query[len(query)-1]
	ql := genQueryLogger(query)

	ql.Duration = duration.Microseconds()
	ql.StartTime = begin
	ql.Hosts = d.Endpoint

	// log the query
	if d.logger != nil {
		d.logger.Debug(ql)
	}

	// push stats to metrics server
	dynamodbStats.WithLabelValues(query[0], ql.Hosts, table).Observe(duration.Seconds())
}

func genPutItemQuery(input *dynamodb.PutItemInput) []string {
	query := []string{"PutItem"}

	query = append(query, fmt.Sprintf("Item Fields %v", getAttributeNames(input.Item)))

	if input.ConditionExpression != nil {
		query = append(query, fmt.Sprintf("ConditionExpression %v", *input.ConditionExpression))
	}

	query = append(query, getTableNameString(input.TableName))

	return query
}

//nolint:goconst // Can't make path suffix as constant
func genGetItemQuery(input *dynamodb.GetItemInput) []string {
	query := []string{"GetItem"}

	if input.AttributesToGet != nil {
		var sub string

		for _, v := range input.AttributesToGet {
			sub += *v + ", "
		}

		sub = strings.TrimSuffix(sub, ", ")

		query = append(query, fmt.Sprintf("AttributesToGet {%v}", sub))
	}

	keys := fmt.Sprintf("Key %v", getAttributeNames(input.Key))
	table := getTableNameString(input.TableName)

	query = append(query, keys, table)

	return query
}

func genDeleteItemQuery(input *dynamodb.DeleteItemInput) []string {
	query := []string{"DeleteItem"}

	if input.ConditionExpression != nil {
		query = append(query, fmt.Sprintf("ConditionExpression %v", *input.ConditionExpression))
	}

	keys := fmt.Sprintf("Key %v", getAttributeNames(input.Key))
	table := getTableNameString(input.TableName)

	query = append(query, keys, table)

	return query
}

func genUpdateItemQuery(input *dynamodb.UpdateItemInput) []string {
	query := []string{"UpdateItem"}

	if input.AttributeUpdates != nil {
		var attributes string

		for key := range input.AttributeUpdates {
			attributes += key + ", "
		}

		attributes = strings.TrimSuffix(attributes, ", ")

		query = append(query, fmt.Sprintf("AttributesToUpdate {%v}", attributes))
	}

	if input.UpdateExpression != nil {
		query = append(query, fmt.Sprintf("UpdateExpression %v", *input.UpdateExpression))
	}

	if input.ConditionExpression != nil {
		query = append(query, fmt.Sprintf("ConditionExpression %v", *input.ConditionExpression))
	}

	keys := fmt.Sprintf("Key %v", getAttributeNames(input.Key))
	table := getTableNameString(input.TableName)

	query = append(query, keys, table)

	return query
}

func getAttributeNames(mp map[string]*dynamodb.AttributeValue) string {
	var names string

	for key := range mp {
		names += key + ", "
	}

	names = strings.TrimSuffix(names, ", ")

	return fmt.Sprintf("{%v}", names)
}

func getTableNameString(tableName *string) string {
	var name string

	if tableName != nil {
		name = *tableName
	}

	return name
}

func genQueryLogger(query []string) QueryLogger {
	var (
		ql       QueryLogger
		q        string
		lenQuery = len(query)
		table    = query[lenQuery-1]
	)

	if lenQuery > 1 {
		q = fmt.Sprintf("%v - with %v, on table %v", query[0], strings.Join(query[1:lenQuery-1], ", "), table)
		ql.Query = []string{q}
	}

	ql.DataStore = DynamoStore

	return ql
}

// Request stores an HTTP service request along with a DB QueryLogger to monitor the duration and execution plan of database queries.
type Request struct {
	*request.Request

	// fields to monitor the query
	queryLogger QueryLogger
}

// Send will send the request to dynamodb, returning error if errors are encountered.
func (r *Request) Send() error {
	begin := time.Now()
	err := r.Request.Send()
	duration := time.Since(begin)

	r.queryLogger.StartTime = begin
	r.queryLogger.Duration = duration.Microseconds()

	// log the query
	if r.queryLogger.Logger != nil {
		r.queryLogger.Logger.Debug(r.queryLogger)
	}

	qValues := strings.Split(r.queryLogger.Query[0], " ")
	table := qValues[len(qValues)-1]
	opType := qValues[0]

	// push stats to metrics server
	dynamodbStats.WithLabelValues(opType, r.queryLogger.Hosts, table).Observe(duration.Seconds())

	return err
}
