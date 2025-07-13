package influxdb

import (
	"context"
	"fmt"
	"gofr.dev/pkg/gofr/datasource"
	"log"
	"time"

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

// CreateOrganization implements container.InfluxDBProvider.
func (c *Client) CreateOrganization(ctx context.Context, orgName string) (string, error) {
	if orgName == "" {
		return "", fmt.Errorf("org Name name must not be empty")
	}
	orgAPI := c.client.OrganizationsAPI()
	newOrg, err := orgAPI.CreateOrganizationWithName(ctx, orgName)
	if err != nil {
		return "", err
	}
	return *newOrg.Id, nil
}

// DeleteOrganization implements container.InfluxDBProvider.
func (c *Client) DeleteOrganization(ctx context.Context, orgId string) error {
	if orgId == "" {
		return fmt.Errorf("orgId name must not be empty")
	}
	orgAPI := c.client.OrganizationsAPI()
	err := orgAPI.DeleteOrganizationWithID(ctx, orgId)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListOrganization(ctx context.Context) (orgs map[string]string, err error) {
	orgAPI := c.client.OrganizationsAPI()
	allOrgs, err := orgAPI.GetOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	orgs = make(map[string]string) // Initialize the map
	if allOrgs == nil {
		return orgs, nil
	}
	for _, org := range *allOrgs {
		orgs[*org.Id] = org.Name
	}
	return orgs, nil
}

// CreateBucket implements container.InfluxDBProvider.
func (c *Client) CreateBucket(ctx context.Context, orgId string, bucketName string, retentionPeriod time.Duration) (bucketId string, err error) {

	// Validate input
	if orgId == "" {
		err = fmt.Errorf("organization id must not be empty")
		return
	}
	if bucketName == "" {
		err = fmt.Errorf("bucket name must not be empty")
		return
	}

	bucketsAPI := c.client.BucketsAPI()
	newBucket, err := bucketsAPI.CreateBucketWithNameWithID(ctx, orgId, bucketName)

	if err != nil {
		return
	}
	return *newBucket.Id, nil
}

// DeleteBucket -=implements container.InfluxDBProvider.
func (c *Client) DeleteBucket(ctx context.Context, org, bucketID string) error {
	if bucketID == "" {
		return fmt.Errorf("bucket name must not be empty")
	}
	bucketsAPI := c.client.BucketsAPI()
	err := bucketsAPI.DeleteBucketWithID(ctx, bucketID)
	if err != nil {
		return err
	}
	return nil
}

// HealthCheck implements container.InfluxDBProvider.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := datasource.Health{Details: make(map[string]any)}

	h.Details["Username"] = c.config.Username
	h.Details["Url"] = c.config.Url

	health, err := c.client.Health(ctx)
	if err != nil {
		h.Status = datasource.StatusDown
		return h, err
	}
	h.Status = datasource.StatusUp
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
		return nil, fmt.Errorf("organization name must not be empty")
	}

	bucketsAPI := c.client.BucketsAPI()
	bucketsDomain, err := bucketsAPI.FindBucketsByOrgName(ctx, org)
	if err != nil {
		// Consider logging the error with context for observability
		log.Printf("failed to find buckets for org %q: %v", org, err)
		return nil, fmt.Errorf("failed to list buckets for organization %q: %w", org, err)
	}

	if bucketsDomain == nil {
		// Defensive: treat nil response as empty result
		return nil, nil
	}

	buckets = make(map[string]string) // Initialize the map
	for _, bucket := range *bucketsDomain {
		if bucket.Name != "" {
			//buckets = append(buckets, bucket.Name)
			buckets[*bucket.Id] = bucket.Name
		}
	}
	return buckets, nil
}

// Ping implements container.InfluxDBProvider.
func (c *Client) Ping(ctx context.Context) (bool, error) {
	ping, err := c.client.Ping(ctx)
	if err != nil {
		return false, err
	}
	return ping, nil
}

// Query implements container.InfluxDBProvider.
func (c *Client) Query(ctx context.Context, org string, fluxQuery string) ([]map[string]any, error) {
	panic("unimplemented")
}

// UseLogger sets the logger for the Elasticsearch client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics implements container.InfluxDBProvider.
func (c *Client) UseMetrics(metrics any) {
	panic("unimplemented")
}

// UseTracer implements container.InfluxDBProvider.
func (c *Client) UseTracer(tracer any) {
	panic("unimplemented")
}

// WritePoints implements container.InfluxDBProvider.
func (c *Client) WritePoints(ctx context.Context, bucket string, org string, points []container.InfluxPoint) error {
	panic("unimplemented")
}

// New creates a new InfluxDB client with the provided configuration.
func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

func (c *Client) Connect() {

	c.logger.Debugf("connecting to influxdb at %v", c.config.Url)

	// Create a new client using an InfluxDB server base URL and an authentication token
	c.client = influxdb2.NewClient(
		c.config.Url,
		c.config.Token,
	)

	if _, err := c.HealthCheck(context.Background()); err != nil {
		c.logger.Errorf("InfluxDB health check failed: %v", err)
		return
	}

	c.logger.Logf("connected to influxdb at : %v", c.config.Url)
}
