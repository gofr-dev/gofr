package influxdb

import (
	"context"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// InfluxClient defines the operations required to interact with an InfluxDB instance.
type InfluxClient interface {
	Setup(ctx context.Context, username, password, org, bucket string, retentionPeriodHours int) (*domain.OnboardingResponse, error)
	Health(ctx context.Context) (*domain.HealthCheck, error)
	Ping(ctx context.Context) (bool, error)
	Options() *influxdb2.Options
	WriteAPIBlocking(org, bucket string) api.WriteAPIBlocking
	QueryAPI(org string) api.QueryAPI
	AuthorizationsAPI() api.AuthorizationsAPI
	OrganizationsAPI() api.OrganizationsAPI
	DeleteAPI() api.DeleteAPI
	BucketsAPI() api.BucketsAPI
}

// InfluxOrganizationsAPI provides methods for managing Organizations in a InfluxDB server.
type InfluxOrganizationsAPI interface {
	GetOrganizations(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Organization, error)
	FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error)
	FindOrganizationByID(ctx context.Context, orgID string) (*domain.Organization, error)
	FindOrganizationsByUserID(ctx context.Context, userID string, pagingOptions ...api.PagingOption) (*[]domain.Organization, error)
	CreateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
	CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error)
	UpdateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
	DeleteOrganization(ctx context.Context, org *domain.Organization) error
	DeleteOrganizationWithID(ctx context.Context, orgID string) error
	GetMembers(ctx context.Context, org *domain.Organization) (*[]domain.ResourceMember, error)
	GetMembersWithID(ctx context.Context, orgID string) (*[]domain.ResourceMember, error)
	AddMember(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceMember, error)
	AddMemberWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceMember, error)
	RemoveMember(ctx context.Context, org *domain.Organization, user *domain.User) error
	RemoveMemberWithID(ctx context.Context, orgID, memberID string) error
	GetOwners(ctx context.Context, org *domain.Organization) (*[]domain.ResourceOwner, error)
	GetOwnersWithID(ctx context.Context, orgID string) (*[]domain.ResourceOwner, error)
	AddOwner(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceOwner, error)
	AddOwnerWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceOwner, error)
	RemoveOwner(ctx context.Context, org *domain.Organization, user *domain.User) error
	RemoveOwnerWithID(ctx context.Context, orgID, memberID string) error
}

// BucketsAPI provides methods for managing Buckets in a InfluxDB server.
type BucketsAPI interface {
	// GetBuckets returns all buckets.
	// GetBuckets supports PagingOptions: Offset, Limit, After. Empty pagingOptions means the default paging (first 20 results).
	GetBuckets(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Bucket, error)
	// FindBucketByName returns a bucket found using bucketName.
	FindBucketByName(ctx context.Context, bucketName string) (*domain.Bucket, error)
	// FindBucketByID returns a bucket found using bucketID.
	FindBucketByID(ctx context.Context, bucketID string) (*domain.Bucket, error)
	// FindBucketsByOrgID returns buckets belonging to the organization with ID orgID.
	// FindBucketsByOrgID supports PagingOptions: Offset, Limit, After. Empty pagingOptions means the default paging (first 20 results).
	FindBucketsByOrgID(ctx context.Context, orgID string, pagingOptions ...api.PagingOption) (*[]domain.Bucket, error)
	// FindBucketsByOrgName returns buckets belonging to the organization with name orgName, with the specified paging.
	//  Empty pagingOptions means the default paging (first 20 results).
	FindBucketsByOrgName(ctx context.Context, orgName string, pagingOptions ...api.PagingOption) (*[]domain.Bucket, error)
	// CreateBucket creates a new bucket.
	CreateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error)
	// CreateBucketWithName creates a new bucket with bucketName in organization org, with retention specified in rules.
	// Empty rules means infinite retention.
	CreateBucketWithName(
		ctx context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error)
	// CreateBucketWithNameWithID creates a new bucket with bucketName in organization with orgID, with retention specified in rules.
	// Empty rules means infinite retention.
	CreateBucketWithNameWithID(ctx context.Context, orgID, bucketName string, rules ...domain.RetentionRule) (*domain.Bucket, error)
	// UpdateBucket updates a bucket.
	UpdateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error)
	// DeleteBucket deletes a bucket.
	DeleteBucket(ctx context.Context, bucket *domain.Bucket) error
	// DeleteBucketWithID deletes a bucket with bucketID.
	DeleteBucketWithID(ctx context.Context, bucketID string) error
	// GetMembers returns members of a bucket.
	GetMembers(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceMember, error)
	// GetMembersWithID returns members of a bucket with bucketID.
	GetMembersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceMember, error)
	AddMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceMember, error)
	AddMemberWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceMember, error)
	RemoveMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) error
	RemoveMemberWithID(ctx context.Context, bucketID, memberID string) error
	GetOwners(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceOwner, error)
	GetOwnersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceOwner, error)
	AddOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceOwner, error)
	AddOwnerWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceOwner, error)
	RemoveOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) error
	RemoveOwnerWithID(ctx context.Context, bucketID, memberID string) error
}

type InfluxQueryAPI interface {
	QueryRaw(ctx context.Context, query string, dialect *domain.Dialect) (string, error)
	QueryRawWithParams(ctx context.Context, query string, dialect *domain.Dialect, params any) (string, error)
	Query(ctx context.Context, query string) (*api.QueryTableResult, error)
	QueryWithParams(ctx context.Context, query string, params any) (*api.QueryTableResult, error)
}
