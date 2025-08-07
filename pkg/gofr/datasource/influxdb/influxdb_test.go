package influxdb

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/kataras/iris/v12/x/errors"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func setupDB(t *testing.T, ctrl *gomock.Controller) *Client {
	t.Helper()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockInfluxClient := NewMockInfluxClient(ctrl)

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

func Test_HelthCheckSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.client.(*MockInfluxClient)

	expectedHealth := &domain.HealthCheck{Status: "pass"}
	mockInflux.EXPECT().
		Health(gomock.Any()).
		Return(expectedHealth, nil).
		Times(1)

	_, err := client.HealthCheck(t.Context())
	require.NoError(t, err)
}

func Test_HelthCheckFail(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.client.(*MockInfluxClient)

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
	mockInflux := client.client.(*MockInfluxClient)

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
	mockInflux := client.client.(*MockInfluxClient)

	mockInflux.EXPECT().
		Ping(gomock.Any()).
		Return(false, errors.New("Something Unexptected")).
		Times(1)

	health, err := client.Ping(t.Context())

	require.Error(t, err) // empty organization name
	require.False(t, health)
}

func Test_CreatOrganization(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dummyID := "dfdf"

	testCases := []struct {
		name      string
		orgName   string
		resp      *domain.Organization
		expectErr bool
		err       error
	}{
		{
			name:      "empty orgainzation name",
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
			mockInflux := client.client.(*MockInfluxClient)
			mockInfluxOrgApi := NewMockInfluxOrganizationsAPI(ctrl)

			mockInflux.EXPECT().OrganizationsAPI().
				Return(mockInfluxOrgApi).
				AnyTimes()

			mockInfluxOrgApi.EXPECT().
				CreateOrganizationWithName(gomock.Any(), tt.orgName).
				Return(tt.resp, tt.err).
				AnyTimes()

			newOrgId, err := client.CreateOrganization(t.Context(), tt.orgName)

			if tt.expectErr {
				require.Error(t, err)
				require.Equal(t, err, tt.err)
				require.Empty(t, newOrgId)
			} else {
				require.NoError(t, err)
			}

			// fmt.Println(orgId, err)
		})
	}
}
