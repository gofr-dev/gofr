package influxdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
)

const (
	Url            = "http://localhost:8086"
	Username       = "admin"
	Password       = "admin1234"
	Token          = "F-QFQpmCL9UkR3qyoXnLkzWj03s6m4eCvYgDl1ePfHBf9ph7yxaSgQ6WN0i9giNgRTfONwVMK1f977r_g71oNQ=="
	testOrgName    = "org-test2"
	testBucketName = "bucket-test2"
	testFluxQuery  = "from(bucket:\"bucket-test2\")|> range(start: -1h) |> filter(fn: (r) => r._measurement == \"stat\")"
)

func setupInflux(t *testing.T) *Client {
	t.Helper()
	app := gofr.New()
	client := New(Config{
		Url:      Url,
		Username: Username,
		Password: Password,
		Token:    Token,
	})
	app.AddInfluxDB(client)
	require.Equal(t, client.config.Url, Url)
	require.Equal(t, client.config.Username, Username)
	require.Equal(t, client.config.Password, Password)
	require.Equal(t, client.config.Token, Token)
	return client
}

func TestPing(t *testing.T) {
	ctx := context.Background()
	config := setupInflux(t)
	health, err := config.Ping(ctx)
	require.NoError(t, err) // empty organization name
	require.True(t, health)
}

func creatOrganization(t *testing.T, client *Client, orgName string) (orgID string) {
	t.Helper()
	ctx := context.Background()
	// try creating organization without name
	orgID, err := client.CreateOrganization(ctx, "")
	require.Error(t, err) // empty organization name
	require.Empty(t, orgID)

	// actually creating organization
	orgID, err = client.CreateOrganization(ctx, orgName)
	require.NoError(t, err)
	require.NotEmpty(t, orgID)

	return orgID
}

func listOrganizations(ctx context.Context, t *testing.T, client *Client) map[string]string {
	t.Helper()
	orgs, err := client.ListOrganization(ctx)
	require.NoError(t, err)
	return orgs
}

func listBuckets(ctx context.Context, t *testing.T, client *Client, orgID string) map[string]string {
	t.Helper()
	buckets, err := client.ListBuckets(ctx, orgID)
	require.NoError(t, err)
	return buckets
}

func deleteOrganization(t *testing.T, client *Client, orgID string) {
	t.Helper()
	ctx := context.Background()

	// try deleting empty id organization
	err := client.DeleteOrganization(ctx, "")
	require.Error(t, err)

	// actually deleting the organization
	err = client.DeleteOrganization(ctx, orgID)
	require.NoError(t, err)
}

func createNewBucket(t *testing.T, client *Client, orgID, name string) (bucketID string) {
	t.Helper()
	ctx := context.Background()

	// try creating organization without name
	bucketID, err := client.CreateBucket(ctx, orgID, "")
	require.Error(t, err) // empty organization name
	require.Empty(t, bucketID)

	// actually creating organization
	bucketID, err = client.CreateBucket(ctx, orgID, name)
	require.NoError(t, err)
	require.NotEmpty(t, bucketID)

	return bucketID
}

func deleteBucket(t *testing.T, client *Client, bucketID string) {
	t.Helper()
	ctx := context.Background()

	// try creating delete empty bucket id
	err := client.DeleteBucket(ctx, "")
	require.Error(t, err)

	err = client.DeleteBucket(ctx, bucketID)
	require.NoError(t, err)
}

func TestOrganizationOperations(t *testing.T) {
	ctx := t.Context()
	client := setupInflux(t)

	beforeOrgList := listOrganizations(ctx, t, client) // before create the new organization
	orgID := creatOrganization(t, client, testOrgName)
	afterOrgList := listOrganizations(ctx, t, client) // after create the new organization
	require.Greater(t, len(afterOrgList), len(beforeOrgList))
	deleteOrganization(t, client, orgID)
}

func TestBucketOperations(t *testing.T) {
	ctx := t.Context()
	client := setupInflux(t)
	orgID := creatOrganization(t, client, testOrgName)

	beforeBucketList := listBuckets(ctx, t, client, testOrgName) // before create the new organization
	bucketID := createNewBucket(t, client, orgID, testBucketName)
	afterBucketList := listBuckets(ctx, t, client, testOrgName) // before create the new organization
	require.Greater(t, len(afterBucketList), len(beforeBucketList))

	deleteBucket(t, client, bucketID)
	deleteOrganization(t, client, orgID)
}

func TestWritePoints(t *testing.T) {
	ctx := t.Context()
	client := setupInflux(t)
	orgID := creatOrganization(t, client, testOrgName)
	bucketID := createNewBucket(t, client, orgID, testBucketName)

	err := client.WritePoint(
		ctx,
		orgID,
		bucketID,
		"stat",
		map[string]string{"unit": "temperature"},
		map[string]any{"avg": 24.5, "max": 45.0},
		time.Now(),
	)

	require.NoError(t, err)

	result, err := client.Query(ctx, orgID, testFluxQuery)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	deleteBucket(t, client, bucketID)
	deleteOrganization(t, client, orgID)
}
