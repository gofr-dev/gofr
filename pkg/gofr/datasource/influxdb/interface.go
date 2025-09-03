package influxdb

import (
	"context"

	influxdb "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// client defines the operations required to interact with an InfluxDB instance.
type client interface {
	Setup(ctx context.Context, username, password, org, bucket string, retentionPeriodHours int) (*domain.OnboardingResponse, error)
	Health(ctx context.Context) (*domain.HealthCheck, error)
	Ping(ctx context.Context) (bool, error)
	Options() *influxdb.Options
	WriteAPIBlocking(org, bucket string) api.WriteAPIBlocking
	QueryAPI(org string) api.QueryAPI
	AuthorizationsAPI() api.AuthorizationsAPI
	OrganizationsAPI() api.OrganizationsAPI
	DeleteAPI() api.DeleteAPI
	BucketsAPI() api.BucketsAPI
}

// organization provides methods for managing Organizations in a InfluxDB server.
type organization interface {
	GetOrganizations(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Organization, error)
	FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error)
	CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error)
	DeleteOrganizationWithID(ctx context.Context, orgID string) error
}

// bucket provides methods for managing Buckets in a InfluxDB server.
type bucket interface {
	GetBuckets(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Bucket, error)
	FindBucketByName(ctx context.Context, bucketName string) (*domain.Bucket, error)
	FindBucketByID(ctx context.Context, bucketID string) (*domain.Bucket, error)
	FindBucketsByOrgName(ctx context.Context, orgName string, pagingOptions ...api.PagingOption) (*[]domain.Bucket, error)
	CreateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error)
	CreateBucketWithName(
		ctx context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error)
	CreateBucketWithNameWithID(ctx context.Context, orgID, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error)
	UpdateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error)
	DeleteBucketWithID(ctx context.Context, bucketID string) error
}

type query interface {
	QueryRaw(ctx context.Context, query string, dialect *domain.Dialect) (string, error)
	QueryRawWithParams(ctx context.Context, query string, dialect *domain.Dialect, params any) (string, error)
	Query(ctx context.Context, query string) (*api.QueryTableResult, error)
	QueryWithParams(ctx context.Context, query string, params any) (*api.QueryTableResult, error)
}
