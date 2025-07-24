package influxdb

import (
	"context"
	"errors"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go.opencensus.io/trace"
	"gofr.dev/pkg/gofr/container"
)

// Config holds the configuration for connecting to InfluxDB.
type Config struct {
	Url      string
	Token    string
	Username string
	Password string
}

// Client represents the InfluxDB client.
type Client struct {
	config  Config
	client  influxdb2.Client
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

type HealthInflux struct {
	Url      string
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
	errEmptyOrganizationId   = errors.New("organization id must not be empty")
	errEmptyBucketId         = errors.New("bucket id must not be empty")
	errEmptyBucketName       = errors.New("bucket name must not be empty")
	errFindingBuckets        = errors.New("failed in finding buckets")
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
	orgAPI := c.client.OrganizationsAPI()
	newOrg, err := orgAPI.CreateOrganizationWithName(ctx, orgName)
	if err != nil {
		return "", err
	}
	return *newOrg.Id, nil
}

// DeleteOrganization deletes an organization in InfluxDB using its ID.
// It implements the container.InfluxDBProvider interface.
//
// Parameters:
// - ctx: Context for request cancellation and timeouts.
// - orgId: The ID of the organization to be deleted. Must not be empty.
//
// Returns:
// - err: Error if the organization deletion fails or if orgId is empty.
func (c *Client) DeleteOrganization(ctx context.Context, orgId string) error {
	if orgId == "" {
		return errEmptyOrganizationId
	}
	orgAPI := c.client.OrganizationsAPI()
	err := orgAPI.DeleteOrganizationWithID(ctx, orgId)
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

	orgAPI := c.client.OrganizationsAPI()
	allOrg, err := orgAPI.GetOrganizations(ctx)
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
// - orgId: The ID of the organization in which the bucket will be created.
// - bucketName: The name of the bucket to be created.
//
// Returns:
// - bucketId: The ID of the newly created bucket.
// - err: Error if bucket creation fails.
func (c *Client) CreateBucket(ctx context.Context, orgId string, bucketName string) (bucketId string, err error) {

	// Validate input
	if orgId == "" {
		err = errEmptyOrganizationId
		return
	}
	if bucketName == "" {
		err = errEmptyBucketName
		return
	}

	bucketsAPI := c.client.BucketsAPI()
	newBucket, err := bucketsAPI.CreateBucketWithNameWithID(ctx, orgId, bucketName)

	if err != nil {
		return
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
func (c *Client) DeleteBucket(ctx context.Context, bucketId string) error {
	if bucketId == "" {
		return errEmptyBucketId
	}
	bucketsAPI := c.client.BucketsAPI()
	err := bucketsAPI.DeleteBucketWithID(ctx, bucketId)
	if err != nil {
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
	h.Details["Url"] = c.config.Url

	health, err := c.client.Health(ctx)
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
		return nil, errEmptyOrganizationId
	}

	bucketsAPI := c.client.BucketsAPI()
	bucketsDomain, err := bucketsAPI.FindBucketsByOrgName(ctx, org)
	if err != nil {
		return nil, errFindingBuckets
	}
	if bucketsDomain == nil {
		return nil, nil
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
	ping, err := c.client.Ping(ctx)
	if err != nil {
		c.logger.Errorf("%v", err)
		return false, err
	}
	return ping, nil
}

func (c *Client) Query(ctx context.Context, org string, fluxQuery string) ([]map[string]any, error) {
	panic("unimplemented")
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

func (c *Client) WritePoints(ctx context.Context, bucket string, org string, points []container.InfluxPoint) error {
	panic("unimplemented")
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

	c.logger.Debugf("connecting to influxdb at %v", c.config.Url)

	// Create a new client using an InfluxDB server base URL and an authentication token
	c.client = influxdb2.NewClient(
		c.config.Url,
		c.config.Token,
	)

	if _, err := c.HealthCheck(context.Background()); err != nil {
		c.logger.Errorf("InfluxDB health check failed: %v", err.Error())
		return
	}

	c.logger.Logf("connected to influxdb at : %v", c.config.Url)
}
