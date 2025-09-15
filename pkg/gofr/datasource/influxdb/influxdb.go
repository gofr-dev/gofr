package influxdb

import (
	"context"
	"errors"
	"time"

	influxdb "github.com/influxdata/influxdb-client-go/v2"
	"go.opencensus.io/trace"
)

// Config holds the configuration for connecting to InfluxDB.
type Config struct {
	URL      string
	Token    string
	Username string
	Password string
}

type influx struct {
	client       client
	organization organization
	bucket       bucket
	query        query
}

// Client represents the InfluxDB client.
type Client struct {
	influx  influx
	config  Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

type HealthInflux struct {
	URL      string
	Token    string
	Username string
	Password string
}

const (
	statusDown = "DOWN"
	statusUp   = "UP"
)

var (
	errEmptyOrganizationName = errors.New("organization name must not be empty")
	errEmptyOrganizationID   = errors.New("organization id must not be empty")
	errEmptyBucketID         = errors.New("bucket id must not be empty")
	errEmptyBucketName       = errors.New("bucket name must not be empty")
	errFetchOrganization     = errors.New("failed to fetch all organizations")
	errHealthCheckFailed     = errors.New("influxdb health check failed")
)

// CreateOrganization creates a new organization in InfluxDB with the specified name.
// It implements the container.InfluxDBProvider interface.
//
// Parameters:
// - ctx: Context for request cancellation and timeouts.
// - orgName: The name of the organization to be created. Must not be empty.
//
// Returns:
// - string: The ID of the newly created organization.
// - error: Error if organization creation fails or if orgName is empty.
func (c *Client) CreateOrganization(ctx context.Context, orgName string) (string, error) {
	if orgName == "" {
		return "", errEmptyOrganizationName
	}

	newOrg, err := c.influx.organization.CreateOrganizationWithName(ctx, orgName)
	if err != nil {
		c.logger.Errorf("failed to create new organization with name '%v' %v", orgName, err)
		return "", err
	}

	c.logger.Debugf("organization created with name '%v'", orgName)

	return *newOrg.Id, nil
}

// DeleteOrganization deletes an organization in InfluxDB using its ID.
// It implements the container.InfluxDBProvider interface.
//
// Parameters:
// - ctx: Context for request cancellation and timeouts.
// - orgID: The ID of the organization to be deleted. Must not be empty.
//
// Returns:
// - err: Error if the organization deletion fails or if orgID is empty.
func (c *Client) DeleteOrganization(ctx context.Context, orgID string) error {
	if orgID == "" {
		return errEmptyOrganizationID
	}

	err := c.influx.organization.DeleteOrganizationWithID(ctx, orgID)
	if err != nil {
		return err
	}

	return nil
}

// ListOrganization retrieves all organizations from InfluxDB and returns their IDs and names.
// It implements the container.InfluxDBProvider interface.
//
// Parameters:
// - ctx: Context for request cancellation and timeouts.
//
// Returns:
// - orgs: A map of organization IDs to their corresponding names.
// - err: Error if the API call fails or the organizations cannot be retrieved.
func (c *Client) ListOrganization(ctx context.Context) (map[string]string, error) {
	allOrg, err := c.influx.organization.GetOrganizations(ctx)
	if err != nil {
		return nil, errFetchOrganization
	}

	if allOrg == nil || len(*allOrg) == 0 {
		return map[string]string{}, nil
	}

	orgs := make(map[string]string, len(*allOrg))

	for _, org := range *allOrg {
		if org.Id != nil {
			orgs[*org.Id] = org.Name
		}
	}

	return orgs, nil
}

// CreateBucket creates a new bucket in InfluxDB for the specified organization.
// Parameters:
// - ctx: Context for request cancellation and timeouts.
// - orgID: The ID of the organization in which the bucket will be created.
// - bucketName: The name of the bucket to be created.
//
// Returns:
// - bucketID: The ID of the newly created bucket.
// - err: Error if bucket creation fails.
func (c *Client) CreateBucket(ctx context.Context, orgID, bucketName string) (bucketID string, err error) {
	if orgID == "" {
		return "", errEmptyOrganizationID
	}

	if bucketName == "" {
		return "", errEmptyBucketName
	}

	newBucket, err := c.influx.bucket.CreateBucketWithNameWithID(ctx, orgID, bucketName)
	if err != nil {
		return "", err
	}

	return *newBucket.Id, nil
}

// DeleteBucket deletes a bucket from InfluxDB by its ID.
// Parameters:
// - ctx: Context for request cancellation and timeouts.
// - org: The ID or name of the organization (not used directly in this implementation).
// - bucketID: The ID of the bucket to be deleted. Must not be empty.
//
// Returns:
// - err: Error if the bucket deletion fails or if bucketID is empty.
func (c *Client) DeleteBucket(ctx context.Context, bucketID string) error {
	if bucketID == "" {
		return errEmptyBucketID
	}

	if err := c.influx.bucket.DeleteBucketWithID(ctx, bucketID); err != nil {
		return err
	}

	return nil
}

type Health struct {
	Status  string         `json:"status"`            // "UP" or "DOWN"
	Details map[string]any `json:"details,omitempty"` // extra metadata
}

// HealthCheck retrieves the health status of the InfluxDB instance.
// It implements the container.InfluxDBProvider interface.
//
// Parameters:
// - ctx: Context for request cancellation and timeouts.
//
// Returns:
// - any: A datasource.Health object containing the status and details of the InfluxDB service.
// - err: Error if the health check request fails or the InfluxDB client returns an error.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{Details: make(map[string]any)}
	h.Details["Username"] = c.config.Username
	h.Details["Url"] = c.config.URL

	health, err := c.influx.client.Health(ctx)
	if err != nil {
		h.Status = statusDown
		h.Details["error"] = err.Error()

		return &h, errHealthCheckFailed
	}

	h.Status = statusUp
	h.Details["Name"] = health.Name
	h.Details["Commit"] = health.Commit
	h.Details["Version"] = health.Version
	h.Details["Message"] = health.Message
	h.Details["Checks"] = health.Checks
	h.Details["Status"] = health.Status

	return h, nil
}

/*
ListBuckets retrieves the names of all buckets for a given organization from InfluxDB.
It implements the container.InfluxDBProvider interface.

Parameters:
- ctx: Context for cancellation and deadlines.
- org: Organization name (must not be empty).

Returns:
- A slice of bucket names.
- An error, if any occurred during retrieval.
*/
func (c *Client) ListBuckets(ctx context.Context, org string) (buckets map[string]string, err error) {
	// Validate input
	if org == "" {
		return nil, errEmptyOrganizationName
	}

	bucketsDomain, err := c.influx.bucket.FindBucketsByOrgName(ctx, org)
	if err != nil {
		return nil, err
	}

	buckets = make(map[string]string) // Initialize the map

	for _, bucket := range *bucketsDomain {
		if bucket.Name != "" {
			buckets[*bucket.Id] = bucket.Name
		}
	}

	return buckets, nil
}

// Ping pings the InfluxDB server to check its availability.
// It implements the container.InfluxDBProvider interface.
//
// Parameters:
// - ctx: Context for request cancellation and timeouts.
//
// Returns:
// - bool: True if the InfluxDB server is reachable; false otherwise.
// - err: Error if the ping request fails.
func (c *Client) Ping(ctx context.Context) (bool, error) {
	ping, err := c.influx.client.Ping(ctx)
	if err != nil {
		c.logger.Errorf("%v", err)
		return false, err
	}

	return ping, nil
}

func (c *Client) Query(ctx context.Context, org, fluxQuery string) ([]map[string]any, error) {
	queryAPI := c.influx.client.QueryAPI(org)

	result, err := queryAPI.Query(ctx, fluxQuery)
	if err != nil {
		c.logger.Errorf("InfluxDB Flux Query '%v' failed: %v", fluxQuery, err.Error())

		return nil, err
	}

	var records []map[string]any

	for result.Next() {
		if result.Err() != nil {
			c.logger.Errorf("Error processing InfluxDB Flux Query result: %v", result.Err().Error())

			return nil, result.Err()
		}

		record := make(map[string]any)

		for k, v := range result.Record().Values() {
			record[k] = v
		}

		records = append(records, record)
	}

	if result.Err() != nil {
		c.logger.Errorf("Final error in InfluxDB Flux Query result: %v", result.Err().Error())

		return nil, result.Err()
	}

	return records, nil
}

func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for InfluxDB client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

func (c *Client) WritePoint(ctx context.Context,
	org, bucket, measurement string,
	tags map[string]string, fields map[string]any, timestamp time.Time,
) error {
	p := influxdb.NewPoint(measurement, tags, fields, timestamp)
	writeAPI := c.influx.client.WriteAPIBlocking(org, bucket)

	if err := writeAPI.WritePoint(ctx, p); err != nil {
		c.logger.Errorf("Failed to write point to influxdb: %v", err.Error())
		return err
	}

	return nil
}

// New creates a new InfluxDB client with the provided configuration.
func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

// Connect initializes a new InfluxDB client using the configured URL and authentication token.
// It logs the connection status and performs a health check to verify connectivity.
//
// If the health check fails, it logs an error and exits early without returning an error.
// No parameters or return values.
func (c *Client) Connect() {
	c.logger.Logf("connecting to influxdb at %v", c.config.URL)

	// Create a new client using an InfluxDB server base URL and an authentication token
	c.influx.client = influxdb.NewClient(
		c.config.URL,
		c.config.Token,
	)

	c.influx.organization = NewInfluxdbOrganizationAPI(c.influx.client.OrganizationsAPI())
	c.influx.bucket = NewInfluxdbBucketAPI(c.influx.client.BucketsAPI())
	c.influx.query = c.influx.client.QueryAPI("")

	if _, err := c.HealthCheck(context.Background()); err != nil {
		c.logger.Errorf("InfluxDB health check failed: %v", err.Error())
		return
	}

	c.logger.Logf("connected to influxdb at : %v", c.config.URL)
}
