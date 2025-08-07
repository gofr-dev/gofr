package influxdb

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/kataras/iris/v12/x/errors"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	influxdb_mock "gofr.dev/pkg/gofr/datasource/influxdb/mocks"
)

func setupDB(t *testing.T, ctrl *gomock.Controller) *Client {
	t.Helper()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockInfluxClient := influxdb_mock.NewMockInfluxClient(ctrl)

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
	client.client = mockInfluxClient

	return client
}

func Test_HealthCheckSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.client.(*influxdb_mock.MockInfluxClient)

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
	mockInflux := client.client.(*influxdb_mock.MockInfluxClient)

	expectedHealth := &domain.HealthCheck{Status: "fail"}
	mockInflux.EXPECT().
		Health(gomock.Any()).
		Return(expectedHealth, errors.New("No influxdb found")).
		Times(1)

	_, err := client.HealthCheck(t.Context())
	require.Error(t, err)
}

func Test_PingSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.client.(*influxdb_mock.MockInfluxClient)

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
	mockInflux := client.client.(*influxdb_mock.MockInfluxClient)

	mockInflux.EXPECT().
		Ping(gomock.Any()).
		Return(false, errors.New("Something Unexptected")).
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
			err:       errors.New("failed to create new organization"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockInflux := client.client.(*influxdb_mock.MockInfluxClient)
			mockInfluxOrgAPI := influxdb_mock.NewMockInfluxOrganizationsAPI(ctrl)

			mockInflux.EXPECT().OrganizationsAPI().
				Return(mockInfluxOrgAPI).
				AnyTimes()

			mockInfluxOrgAPI.EXPECT().
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
			err:       errors.New("invalid organization id"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockInflux := client.client.(*influxdb_mock.MockInfluxClient)
			mockInfluxOrgAPI := influxdb_mock.NewMockInfluxOrganizationsAPI(ctrl)

			mockInflux.EXPECT().OrganizationsAPI().
				Return(mockInfluxOrgAPI).
				AnyTimes()

			mockInfluxOrgAPI.EXPECT().
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
	mockInflux := client.client.(*influxdb_mock.MockInfluxClient)
	mockInfluxOrgAPI := influxdb_mock.NewMockInfluxOrganizationsAPI(ctrl)

	t.Run("test zero organization", func(t *testing.T) {
		allOrgs := []domain.Organization{}

		mockInflux.EXPECT().OrganizationsAPI().Return(mockInfluxOrgAPI).Times(1)
		mockInfluxOrgAPI.EXPECT().
			GetOrganizations(gomock.Any()).
			Return(&allOrgs, nil).
			Times(1)

		orgs, err := client.ListOrganization(t.Context())
		require.NoError(t, err)
		require.Empty(t, orgs)
	})

	t.Run("testing error in fetching organization", func(t *testing.T) {
		allOrgs := &[]domain.Organization{}

		mockInflux.EXPECT().OrganizationsAPI().Return(mockInfluxOrgAPI).Times(1)
		mockInfluxOrgAPI.EXPECT().
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

		mockInflux.EXPECT().OrganizationsAPI().Return(mockInfluxOrgAPI).Times(1)
		mockInfluxOrgAPI.EXPECT().
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

	dummyId := "id1"
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
			respBucket:   &domain.Bucket{Id: &dummyId},
			wantBucketID: dummyId,
			err:          nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockInflux := client.client.(*influxdb_mock.MockInfluxClient)
			mockInfluxBucketAPI := influxdb_mock.NewMockBucketsAPI(ctrl)

			mockInflux.EXPECT().BucketsAPI().
				Return(mockInfluxBucketAPI).
				AnyTimes()

			mockInfluxBucketAPI.EXPECT().
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

	dummyId := "id1"

	testCases := []struct {
		name      string
		orgID     string
		bucketId  string
		expectErr bool
		err       error
	}{
		{
			name:      "try deleting bucket with empty bucket id",
			orgID:     "",
			bucketId:  "",
			expectErr: true,
			err:       errEmptyBucketID,
		},
		{
			name:      "successfully deleting a new bucket",
			orgID:     "",
			bucketId:  dummyId,
			expectErr: false,
			err:       nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockInflux := client.client.(*influxdb_mock.MockInfluxClient)
			mockInfluxBucketAPI := influxdb_mock.NewMockBucketsAPI(ctrl)

			mockInflux.EXPECT().BucketsAPI().
				Return(mockInfluxBucketAPI).
				AnyTimes()

			mockInfluxBucketAPI.EXPECT().
				DeleteBucketWithID(gomock.Any(), tt.bucketId).
				Return(tt.err).
				AnyTimes()

			err := client.DeleteBucket(t.Context(), tt.bucketId)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
