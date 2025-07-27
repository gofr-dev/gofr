package influxdb

import (
	"context"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr"
	"testing"
	"time"
)

var (
	Url            = "http://localhost:8086"
	Username       = "admin"
	Password       = "admin1234"
	Token          = "F-QFQpmCL9UkR3qyoXnLkzWj03s6m4eCvYgDl1ePfHBf9ph7yxaSgQ6WN0i9giNgRTfONwVMK1f977r_g71oNQ=="
	testOrgName    = "org-test2"
	testBucketName = "bucket-test2"
	testFluxQuery  = "from(bucket:\"bucket-test2\")|> range(start: -1h) |> filter(fn: (r) => r._measurement == \"stat\")"
)

func setupInflux(t *testing.T) *Client {
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
	require.Equal(t, health, true)
}

func creatOrganization(t *testing.T, client *Client, orgName string) (orgId string) {
	ctx := context.Background()

	// try creating organization without name
	orgId, err := client.CreateOrganization(ctx, "")
	require.Error(t, err) // empty organization name
	require.Equal(t, "", orgId)

	// actually creating organization
	orgId, err = client.CreateOrganization(ctx, orgName)
	require.NoError(t, err)
	require.NotEmpty(t, orgId)

	return orgId
}

func listOrganizations(t *testing.T, ctx context.Context, client *Client) map[string]string {
	orgs, err := client.ListOrganization(ctx)
	require.NoError(t, err)
	return orgs
}

func listBuckets(t *testing.T, ctx context.Context, client *Client, orgId string) map[string]string {
	buckets, err := client.ListBuckets(ctx, orgId)
	require.NoError(t, err)
	return buckets
}

func deleteOrganization(t *testing.T, client *Client, orgId string) {
	ctx := context.Background()

	// try deleting empty id organization
	err := client.DeleteOrganization(ctx, "")
	require.Error(t, err)

	// actually deleting the organization
	err = client.DeleteOrganization(ctx, orgId)
	require.NoError(t, err)
}

func createNewBucket(t *testing.T, client *Client, orgId string, name string) (bucketId string) {
	ctx := context.Background()

	// try creating organization without name
	bucketId, err := client.CreateBucket(ctx, orgId, "")
	require.Error(t, err) // empty organization name
	require.Empty(t, bucketId)

	// actually creating organization
	bucketId, err = client.CreateBucket(ctx, orgId, name)
	require.NoError(t, err)
	require.NotEmpty(t, bucketId)

	return bucketId
}

func deleteBucket(t *testing.T, client *Client, bucketId string) {
	ctx := context.Background()

	// try creating delete empty bucket id
	err := client.DeleteBucket(ctx, "")
	require.Error(t, err)

	err = client.DeleteBucket(ctx, bucketId)
	require.NoError(t, err)
}

func TestOrganizationOperations(t *testing.T) {
	ctx := t.Context()
	client := setupInflux(t)

	beforeOrgList := listOrganizations(t, ctx, client) // before create the new organization
	orgId := creatOrganization(t, client, testOrgName)
	afterOrgList := listOrganizations(t, ctx, client) // after create the new organization
	require.Greater(t, len(afterOrgList), len(beforeOrgList))
	deleteOrganization(t, client, orgId)
}

func TestBucketOperations(t *testing.T) {

	ctx := t.Context()
	client := setupInflux(t)
	orgId := creatOrganization(t, client, testOrgName)

	beforeBucketList := listBuckets(t, ctx, client, testOrgName) // before create the new organization
	bucketId := createNewBucket(t, client, orgId, testBucketName)
	afterBucketList := listBuckets(t, ctx, client, testOrgName) // before create the new organization
	require.Greater(t, len(afterBucketList), len(beforeBucketList))

	deleteBucket(t, client, bucketId)
	deleteOrganization(t, client, orgId)
}

func TestWritePoints(t *testing.T) {
	ctx := t.Context()
	client := setupInflux(t)
	orgId := creatOrganization(t, client, testOrgName)
	bucketId := createNewBucket(t, client, orgId, testBucketName)

	err := client.WritePoint(
		ctx,
		orgId,
		bucketId,
		"stat",
		map[string]string{"unit": "temperature"},
		map[string]interface{}{"avg": 24.5, "max": 45.0},
		time.Now(),
	)

	require.NoError(t, err)

	result, err := client.Query(ctx, orgId, testFluxQuery)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	deleteBucket(t, client, bucketId)
	deleteOrganization(t, client, orgId)
}
