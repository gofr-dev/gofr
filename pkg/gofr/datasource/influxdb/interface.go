package influxdb

import (
	"context"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// InfluxDB defines the operations required to interact with an InfluxDB instance.
type InfluxClient interface {

	// Setup sends request to initialise new InfluxDB server with user, org and bucket, and data retention period
	// and returns details about newly created entities along with the authorization object.
	// Retention period of zero will result to infinite retention.
	Setup(ctx context.Context, username, password, org, bucket string, retentionPeriodHours int) (*domain.OnboardingResponse, error)
	// SetupWithToken sends request to initialise new InfluxDB server with user, org and bucket, data retention period and token
	// and returns details about newly created entities along with the authorization object.
	// Retention period of zero will result to infinite retention.
	SetupWithToken(ctx context.Context, username, password, org, bucket string, retentionPeriodHours int, token string) (*domain.OnboardingResponse, error)
	// Ready returns InfluxDB uptime info of server. It doesn't validate authentication params.
	Ready(ctx context.Context) (*domain.Ready, error)
	// Health returns an InfluxDB server health check result. Read the HealthCheck.Status field to get server status.
	// Health doesn't validate authentication params.
	Health(ctx context.Context) (*domain.HealthCheck, error)
	// Ping validates whether InfluxDB server is running. It doesn't validate authentication params.
	Ping(ctx context.Context) (bool, error)
	// Close ensures all ongoing asynchronous write clients finish.
	// Also closes all idle connections, in case of HTTP client was created internally.
	Close()
	// Options returns the options associated with client
	Options() *influxdb2.Options
	// ServerURL returns the url of the server url client talks to
	ServerURL() string
	// HTTPService returns underlying HTTP service object used by client
	HTTPService() http.Service
	// WriteAPI returns the asynchronous, non-blocking, Write client.
	// Ensures using a single WriteAPI instance for each org/bucket pair.
	WriteAPI(org, bucket string) api.WriteAPI
	// WriteAPIBlocking returns the synchronous, blocking, Write client.
	// Ensures using a single WriteAPIBlocking instance for each org/bucket pair.
	WriteAPIBlocking(org, bucket string) api.WriteAPIBlocking
	// QueryAPI returns Query client.
	// Ensures using a single QueryAPI instance each org.
	QueryAPI(org string) api.QueryAPI
	// AuthorizationsAPI returns Authorizations API client.
	AuthorizationsAPI() api.AuthorizationsAPI
	// OrganizationsAPI returns Organizations API client
	OrganizationsAPI() api.OrganizationsAPI
	// UsersAPI returns Users API client.
	UsersAPI() api.UsersAPI
	// DeleteAPI returns Delete API client
	DeleteAPI() api.DeleteAPI
	// BucketsAPI returns Buckets API client
	BucketsAPI() api.BucketsAPI
	// LabelsAPI returns Labels API client
	LabelsAPI() api.LabelsAPI
	// TasksAPI returns Tasks API client
	TasksAPI() api.TasksAPI

	APIClient() *domain.Client
}
