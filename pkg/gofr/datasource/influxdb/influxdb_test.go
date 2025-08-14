package influxdb

import (
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	influxdb_mock "gofr.dev/pkg/gofr/datasource/influxdb/mocks"
)

var (
	errInvalidOrgID      = errors.New("invalid organization id")
	errFailedCreatingOrg = errors.New("failed to create new organization")
	errPingFailed        = errors.New("failed to ping")
)

func setupDB(t *testing.T, ctrl *gomock.Controller) *Client {
	t.Helper()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)

	config := Config{
		URL:      "http://localhost:8086",
		Username: "username",
		Password: "password",
		Token:    "token",
	}

	client := New(config)

	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-influxdb"))

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Replace the client with our mocked version
	client.influx.client = influxdb_mock.NewMockclient(ctrl)

	return client
}

func Test_HealthCheckSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.influx.client.(*influxdb_mock.Mockclient)

	expectedHealth := &domain.HealthCheck{Status: "pass"}
	mockInflux.EXPECT().
		Health(gomock.Any()).
		Return(expectedHealth, nil).
		Times(1)

	_, err := client.HealthCheck(t.Context())
	require.NoError(t, err)
}

func Test_HealthCheckFail(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.influx.client.(*influxdb_mock.Mockclient)

	expectedHealth := &domain.HealthCheck{Status: "fail"}
	mockInflux.EXPECT().
		Health(gomock.Any()).
		Return(expectedHealth, errEmptyBucketID).
		Times(1)

	_, err := client.HealthCheck(t.Context())
	require.Error(t, err)
}

func Test_PingSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.influx.client.(*influxdb_mock.Mockclient)

	mockInflux.EXPECT().
		Ping(gomock.Any()).
		Return(true, nil).
		Times(1)

	health, err := client.Ping(t.Context())

	require.NoError(t, err) // empty organization name
	require.True(t, health)
}

func Test_PingFailed(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.influx.client.(*influxdb_mock.Mockclient)

	mockInflux.EXPECT().
		Ping(gomock.Any()).
		Return(false, errPingFailed).
		Times(1)

	health, err := client.Ping(t.Context())

	require.Error(t, err) // empty organization name
	require.False(t, health)
}

func Test_CreateOrganization(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dummyID := "dummyID"

	testCases := []struct {
		name      string
		orgName   string
		resp      *domain.Organization
		expectErr bool
		err       error
	}{
		{
			name:      "empty organizations name",
			orgName:   "",
			resp:      &domain.Organization{},
			expectErr: true,
			err:       errEmptyOrganizationName,
		},
		{
			name:    "create new organization",
			orgName: "testOrg",
			resp: &domain.Organization{
				Id: &dummyID,
			},
			expectErr: false,
			err:       nil,
		},
		{
			name:      "create duplicate organization",
			orgName:   "testOrg",
			resp:      &domain.Organization{},
			expectErr: true,
			err:       errFailedCreatingOrg,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockOrganization := influxdb_mock.NewMockorganization(ctrl)

			client.influx.organization = mockOrganization
			mockOrganization.EXPECT().
				CreateOrganizationWithName(gomock.Any(), tt.orgName).
				Return(tt.resp, tt.err).
				AnyTimes()

			newOrgID, err := client.CreateOrganization(t.Context(), tt.orgName)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
				require.Empty(t, newOrgID)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_DeleteOrganization(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dummyID := "dummyID"

	testCases := []struct {
		name      string
		orgID     string
		expectErr bool
		err       error
	}{
		{
			name:      "delete empty organizations with id",
			orgID:     "",
			expectErr: true,
			err:       errEmptyOrganizationID,
		},
		{
			name:      "delete organization with id",
			orgID:     dummyID,
			expectErr: false,
			err:       nil,
		},
		{
			name:      "delete invalid organization with id",
			orgID:     dummyID,
			expectErr: true,
			err:       errInvalidOrgID,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockOrganization := influxdb_mock.NewMockorganization(ctrl)
			client.influx.organization = mockOrganization

			mockOrganization.EXPECT().
				DeleteOrganizationWithID(gomock.Any(), tt.orgID).
				Return(tt.err).
				AnyTimes()

			err := client.DeleteOrganization(t.Context(), tt.orgID)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ListOrganization(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockOrganization := influxdb_mock.NewMockorganization(ctrl)
	client.influx.organization = mockOrganization

	t.Run("test zero organization", func(t *testing.T) {
		allOrgs := []domain.Organization{}

		// mockInflux.EXPECT().OrganizationsAPI().Return(mockOrganization).Times(1)
		mockOrganization.EXPECT().
			GetOrganizations(gomock.Any()).
			Return(&allOrgs, nil).
			Times(1)

		orgs, err := client.ListOrganization(t.Context())
		require.NoError(t, err)
		require.Empty(t, orgs)
	})

	t.Run("testing error in fetching organization", func(t *testing.T) {
		allOrgs := &[]domain.Organization{}

		mockOrganization.EXPECT().
			GetOrganizations(gomock.Any()).
			Return(allOrgs, errFetchOrganization).
			Times(1)

		orgs, err := client.ListOrganization(t.Context())
		require.Empty(t, orgs)
		require.Error(t, err)
		require.Equal(t, err, errFetchOrganization)
	})

	t.Run("testing fetching list of organization", func(t *testing.T) {
		id1, name1 := "id1", "name1"
		id2, name2 := "id1", "name1"

		allOrg := &[]domain.Organization{
			{Id: &id1, Name: name1},
			{Id: &id2, Name: name2},
		}

		wantOrg := map[string]string{id1: name1, id2: name2}

		mockOrganization.EXPECT().
			GetOrganizations(gomock.Any()).
			Return(allOrg, nil).
			Times(1)

		resultOrg, err := client.ListOrganization(t.Context())

		require.NoError(t, err)
		require.NotEmpty(t, resultOrg)

		orgs := make(map[string]string, len(*allOrg))

		for _, org := range *allOrg {
			orgs[*org.Id] = org.Name
		}

		require.Equal(t, wantOrg, orgs)
	})
}

func Test_CreateBucket(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dummyID := "id1"
	dummyOrgID := "org123"
	dummyBucketName := "bucket123"

	testCases := []struct {
		name         string
		orgID        string
		bucketName   string
		respBucket   *domain.Bucket
		wantBucketID string
		expectErr    bool
		err          error
	}{
		{
			name:         "try creating bucket with empty organization id",
			orgID:        "",
			bucketName:   dummyBucketName,
			expectErr:    true,
			respBucket:   nil,
			wantBucketID: "",
			err:          errEmptyOrganizationID,
		},
		{
			name:         "try creating bucket with empty bucket name",
			orgID:        dummyOrgID,
			bucketName:   "",
			expectErr:    true,
			respBucket:   nil,
			wantBucketID: "",
			err:          errEmptyBucketName,
		},
		{
			name:         "successfully creating a new bucket",
			orgID:        dummyOrgID,
			bucketName:   dummyBucketName,
			expectErr:    false,
			respBucket:   &domain.Bucket{Id: &dummyID},
			wantBucketID: dummyID,
			err:          nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)

			mockBucket := influxdb_mock.NewMockbucket(ctrl)

			client.influx.bucket = mockBucket
			mockBucket.EXPECT().
				CreateBucketWithNameWithID(gomock.Any(), tt.orgID, tt.bucketName).
				Return(tt.respBucket, tt.err).
				AnyTimes()

			bucketID, err := client.CreateBucket(t.Context(), tt.orgID, tt.bucketName)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
			} else {
				require.Equal(t, tt.wantBucketID, bucketID)
				require.NoError(t, err)
			}
		})
	}
}

func Test_DeleteBucket(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dummyID := "id1"

	testCases := []struct {
		name      string
		orgID     string
		bucketID  string
		expectErr bool
		err       error
	}{
		{
			name:      "try deleting bucket with empty bucket id",
			orgID:     "",
			bucketID:  "",
			expectErr: true,
			err:       errEmptyBucketID,
		},
		{
			name:      "successfully deleting a new bucket",
			orgID:     "",
			bucketID:  dummyID,
			expectErr: false,
			err:       nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockBucket := influxdb_mock.NewMockbucket(ctrl)

			client.influx.bucket = mockBucket
			mockBucket.EXPECT().
				DeleteBucketWithID(gomock.Any(), tt.bucketID).
				Return(tt.err).
				AnyTimes()

			err := client.DeleteBucket(t.Context(), tt.bucketID)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_ListBucket(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dummyOrgName := "orgName"
	id1, name1 := "id1", "name1"
	id2, name2 := "id1", "name1"

	testCases := []struct {
		name        string
		orgName     string
		resp        *[]domain.Bucket
		wantBuckets map[string]string
		expectErr   bool
		err         error
	}{
		{
			name:        "try list bucket with empty organization name",
			orgName:     "",
			expectErr:   true,
			wantBuckets: nil,
			resp:        &[]domain.Bucket{},
			err:         errEmptyOrganizationName,
		},

		{
			name:    "success list organizations",
			orgName: dummyOrgName,
			resp: &[]domain.Bucket{
				{Id: &id1, Name: name1},
				{Id: &id2, Name: name2},
			},
			wantBuckets: map[string]string{
				id1: name1,
				id2: name2,
			},
			expectErr: false,
			err:       nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)

			mockBucket := influxdb_mock.NewMockbucket(ctrl)

			client.influx.bucket = mockBucket
			mockBucket.EXPECT().
				FindBucketsByOrgName(gomock.Any(), tt.orgName).
				Return(tt.resp, tt.err).
				AnyTimes()

			buckets, err := client.ListBuckets(t.Context(), tt.orgName)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, buckets)
				require.Equal(t, tt.wantBuckets, buckets)
			}
		})
	}
}

// func Test_Query(t *testing.T) {
// 	t.Helper()

// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	testCases := []struct {
// 		name        string
// 		resp        *api.QueryTableResult
// 		wantResults map[string]any
// 		orgName     string
// 		inputQuery  string
// 		expectErr   bool
// 		err         error
// 	}{
// 		{
// 			name:        "failing error query",
// 			orgName:     "org1",
// 			inputQuery:  "dummyQuery1",
// 			expectErr:   true,
// 			wantResults: map[string]any{},
// 			resp:        &api.QueryTableResult{},
// 			err:         errors.New("Something"),
// 		},
// 	}

// 	for _, tt := range testCases {
// 		t.Run(tt.name, func(t *testing.T) {
// 			client := *setupDB(t, ctrl)
// 			mockInflux := client.client.(*influxdb_mock.MockInfluxClient)
// 			mockInfluxQueryAPI := influxdb_mock.NewMockInfluxQueryAPI(ctrl)

// 			mockInflux.EXPECT().QueryAPI(tt.orgName).
// 				Return(mockInfluxQueryAPI).
// 				AnyTimes()

// 			mockInfluxQueryAPI.EXPECT().
// 				Query(gomock.Any(), tt.inputQuery).
// 				Return(tt.resp, tt.err).
// 				AnyTimes()

// 			result, err := client.Query(t.Context(), tt.orgName, tt.inputQuery)

// 			if tt.expectErr {
// 				require.Error(t, err)
// 				require.Equal(t, err, tt.err)
// 				require.Empty(t, result)
// 			} else {
// 				require.NoError(t, err)
// 			}
// 		})
// 	}
// }
