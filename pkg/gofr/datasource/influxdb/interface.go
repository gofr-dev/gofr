package influxdb

import (
	"context"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// InfluxClient defines the operations required to interact with an InfluxDB instance.
type InfluxClient interface {

	// Setup sends request to initialize new InfluxDB server with user, org and bucket, and data retention period
	// and returns details about newly created entities along with the authorization object.
	// Retention period of zero will result to infinite retention.
	Setup(ctx context.Context, username, password, org, bucket string, retentionPeriodHours int) (*domain.OnboardingResponse, error)
	// SetupWithToken sends request to initialize new InfluxDB server with user, org and bucket, data retention period and token
	// and returns details about newly created entities along with the authorization object.
	// Retention period of zero will result to infinite retention.
	SetupWithToken(
		ctx context.Context,
		username, password, org, bucket string,
		retentionPeriodHours int, token string) (*domain.OnboardingResponse, error)

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

// InfluxOrganizationsAPI provides methods for managing Organizations in a InfluxDB server.
type InfluxOrganizationsAPI interface {
	// GetOrganizations returns all organizations.
	// GetOrganizations supports PagingOptions: Offset, Limit, Descending
	GetOrganizations(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Organization, error)
	// FindOrganizationByName returns an organization found using orgName.
	FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error)
	// FindOrganizationByID returns an organization found using orgID.
	FindOrganizationByID(ctx context.Context, orgID string) (*domain.Organization, error)
	// FindOrganizationsByUserID returns organizations an user with userID belongs to.
	// FindOrganizationsByUserID supports PagingOptions: Offset, Limit, Descending
	FindOrganizationsByUserID(ctx context.Context, userID string, pagingOptions ...api.PagingOption) (*[]domain.Organization, error)
	// CreateOrganization creates new organization.
	CreateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
	// CreateOrganizationWithName creates new organization with orgName and with status active.
	CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error)
	// UpdateOrganization updates organization.
	UpdateOrganization(ctx context.Context, org *domain.Organization) (*domain.Organization, error)
	// DeleteOrganization deletes an organization.
	DeleteOrganization(ctx context.Context, org *domain.Organization) error
	// DeleteOrganizationWithID deletes an organization with orgID.
	DeleteOrganizationWithID(ctx context.Context, orgID string) error
	// GetMembers returns members of an organization.
	GetMembers(ctx context.Context, org *domain.Organization) (*[]domain.ResourceMember, error)
	// GetMembersWithID returns members of an organization with orgID.
	GetMembersWithID(ctx context.Context, orgID string) (*[]domain.ResourceMember, error)
	// AddMember adds a member to an organization.
	AddMember(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceMember, error)
	// AddMemberWithID adds a member with id memberID to an organization with orgID.
	AddMemberWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceMember, error)
	// RemoveMember removes a member from an organization.
	RemoveMember(ctx context.Context, org *domain.Organization, user *domain.User) error
	// RemoveMemberWithID removes a member with id memberID from an organization with orgID.
	RemoveMemberWithID(ctx context.Context, orgID, memberID string) error
	// GetOwners returns owners of an organization.
	GetOwners(ctx context.Context, org *domain.Organization) (*[]domain.ResourceOwner, error)
	// GetOwnersWithID returns owners of an organization with orgID.
	GetOwnersWithID(ctx context.Context, orgID string) (*[]domain.ResourceOwner, error)
	// AddOwner adds an owner to an organization.
	AddOwner(ctx context.Context, org *domain.Organization, user *domain.User) (*domain.ResourceOwner, error)
	// AddOwnerWithID adds an owner with id memberID to an organization with orgID.
	AddOwnerWithID(ctx context.Context, orgID, memberID string) (*domain.ResourceOwner, error)
	// RemoveOwner removes an owner from an organization.
	RemoveOwner(ctx context.Context, org *domain.Organization, user *domain.User) error
	// RemoveOwnerWithID removes an owner with id memberID from an organization with orgID.
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
	// AddMember adds a member to a bucket.
	AddMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceMember, error)
	// AddMemberWithID adds a member with id memberID to a bucket with bucketID.
	AddMemberWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceMember, error)
	// RemoveMember removes a member from a bucket.
	RemoveMember(ctx context.Context, bucket *domain.Bucket, user *domain.User) error
	// RemoveMemberWithID removes a member with id memberID from a bucket with bucketID.
	RemoveMemberWithID(ctx context.Context, bucketID, memberID string) error
	// GetOwners returns owners of a bucket.
	GetOwners(ctx context.Context, bucket *domain.Bucket) (*[]domain.ResourceOwner, error)
	// GetOwnersWithID returns owners of a bucket with bucketID.
	GetOwnersWithID(ctx context.Context, bucketID string) (*[]domain.ResourceOwner, error)
	// AddOwner adds an owner to a bucket.
	AddOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) (*domain.ResourceOwner, error)
	// AddOwnerWithID adds an owner with id memberID to a bucket with bucketID.
	AddOwnerWithID(ctx context.Context, bucketID, memberID string) (*domain.ResourceOwner, error)
	// RemoveOwner removes an owner from a bucket.
	RemoveOwner(ctx context.Context, bucket *domain.Bucket, user *domain.User) error
	// RemoveOwnerWithID removes a member with id memberID from a bucket with bucketID.
	RemoveOwnerWithID(ctx context.Context, bucketID, memberID string) error
}
