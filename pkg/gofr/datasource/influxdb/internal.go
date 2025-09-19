package influxdb

import (
	"context"

	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

type influxdbOrganizationAPI struct {
	api api.OrganizationsAPI
}

// NewInfluxdbOrganizationAPI creates a new bucket API wrapper.
func NewInfluxdbOrganizationAPI(a api.OrganizationsAPI) organization {
	return influxdbOrganizationAPI{api: a}
}

func (a influxdbOrganizationAPI) GetOrganizations(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Organization, error) {
	return a.api.GetOrganizations(ctx, pagingOptions...)
}

func (a influxdbOrganizationAPI) FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error) {
	return a.api.FindOrganizationByName(ctx, orgName)
}

func (a influxdbOrganizationAPI) CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error) {
	return a.api.CreateOrganizationWithName(ctx, orgName)
}

func (a influxdbOrganizationAPI) DeleteOrganizationWithID(ctx context.Context, orgID string) error {
	return a.api.DeleteOrganizationWithID(ctx, orgID)
}

// influxdbBucketAPI.
type influxdbBucketAPI struct {
	api api.BucketsAPI
}

// NewInfluxdbBucketAPI creates a new bucket API wrapper.
func NewInfluxdbBucketAPI(a api.BucketsAPI) bucket {
	return influxdbBucketAPI{api: a}
}

func (b influxdbBucketAPI) GetBuckets(ctx context.Context, pagingOptions ...api.PagingOption) (*[]domain.Bucket, error) {
	return b.api.GetBuckets(ctx, pagingOptions...)
}

func (b influxdbBucketAPI) FindBucketsByOrgName(ctx context.Context, orgName string, pagingOptions ...api.PagingOption) (
	*[]domain.Bucket, error,
) {
	return b.api.FindBucketsByOrgName(ctx, orgName, pagingOptions...)
}

func (b influxdbBucketAPI) FindBucketByName(ctx context.Context, bucketName string) (*domain.Bucket, error) {
	return b.api.FindBucketByName(ctx, bucketName)
}

func (b influxdbBucketAPI) FindBucketByID(ctx context.Context, bucketID string) (*domain.Bucket, error) {
	return b.api.FindBucketByID(ctx, bucketID)
}

func (b influxdbBucketAPI) CreateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	return b.api.CreateBucket(ctx, bucket)
}

func (b influxdbBucketAPI) CreateBucketWithName(
	ctx context.Context, org *domain.Organization, bucketName string, rules ...domain.RetentionRule,
) (*domain.Bucket, error) {
	return b.api.CreateBucketWithName(ctx, org, bucketName, rules...)
}

func (b influxdbBucketAPI) CreateBucketWithNameWithID(
	ctx context.Context, orgID, bucketName string, rules ...domain.RetentionRule,
) (*domain.Bucket, error) {
	return b.api.CreateBucketWithNameWithID(ctx, orgID, bucketName, rules...)
}

func (b influxdbBucketAPI) UpdateBucket(ctx context.Context, bucket *domain.Bucket) (*domain.Bucket, error) {
	return b.api.UpdateBucket(ctx, bucket)
}

func (b influxdbBucketAPI) DeleteBucketWithID(ctx context.Context, bucketID string) error {
	return b.api.DeleteBucketWithID(ctx, bucketID)
}
