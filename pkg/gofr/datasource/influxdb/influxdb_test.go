package influxdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/otel"
)

var (
	errInvalidOrgID      = errors.New("invalid organization id")
	errFailedCreatingOrg = errors.New("failed to create new organization")
	errPingFailed        = errors.New("failed to ping")
	errFailedQuery       = errors.New("error failed query")
	errWritePoint        = errors.New("failed to write point")
	errBucketOp          = errors.New("bucket operation failed")
	errOrgOp             = errors.New("organization operation failed")
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

	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Replace the client with our mocked version
	client.influx.client = NewMockclient(ctrl)

	return client
}

func Test_HealthCheckSuccess(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockInflux := client.influx.client.(*Mockclient)

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
	mockInflux := client.influx.client.(*Mockclient)

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
	mockInflux := client.influx.client.(*Mockclient)

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
	mockInflux := client.influx.client.(*Mockclient)

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
			mockOrganization := NewMockorganization(ctrl)

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
			mockOrganization := NewMockorganization(ctrl)
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
	mockOrganization := NewMockorganization(ctrl)
	client.influx.organization = mockOrganization

	t.Run("test zero organization", func(t *testing.T) {
		allOrgs := []domain.Organization{}

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

			mockBucket := NewMockbucket(ctrl)

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
			mockBucket := NewMockbucket(ctrl)

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

			mockBucket := NewMockbucket(ctrl)

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

func Test_Query(t *testing.T) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := *setupDB(t, ctrl)
	mockClient := client.influx.client.(*Mockclient)
	mockQueryAPI := NewMockquery(ctrl)

	// Mock QueryAPI call
	mockClient.EXPECT().
		QueryAPI("org1").
		Return(mockQueryAPI).
		Times(1)

	// Mock Query call
	mockQueryAPI.EXPECT().
		Query(gomock.Any(), "dummyQuery1").
		Return(nil, errFailedQuery).
		Times(1)

	result, err := client.Query(t.Context(), "org1", "dummyQuery1")

	require.Error(t, err)
	require.Equal(t, errFailedQuery, err)
	require.Empty(t, result)
}

const validQueryCSV = `#datatype,string,long,dateTime:RFC3339,double,string,string
#group,false,false,false,false,true,true
#default,_result,,,,,
,result,table,_time,_value,_field,_measurement
,,0,2020-02-18T10:34:08.135814545Z,1.4,f,test
`

func Test_QueryResults(t *testing.T) {
	ctrl := gomock.NewController(t)

	ts, err := time.Parse(time.RFC3339Nano, "2020-02-18T10:34:08.135814545Z")
	require.NoError(t, err)

	tests := []struct {
		desc      string
		body      string
		expResult []map[string]any
		expErr    string
	}{
		{
			desc: "successful query with one record",
			body: validQueryCSV,
			expResult: []map[string]any{{
				"result": "_result", "table": int64(0), "_time": ts, "_value": 1.4,
				"_field": "f", "_measurement": "test",
			}},
			expErr: "<nil>",
		},
		{
			desc:      "empty result",
			body:      "",
			expResult: nil,
			expErr:    "<nil>",
		},
		{
			desc:   "result without annotations returns error",
			body:   ",result,table\n,,0\n",
			expErr: "parsing error, annotations not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockClient := client.influx.client.(*Mockclient)
			mockQueryAPI := NewMockquery(ctrl)

			mockClient.EXPECT().QueryAPI("org1").Return(mockQueryAPI)
			mockQueryAPI.EXPECT().Query(gomock.Any(), "from(bucket: \"b\")").
				Return(api.NewQueryTableResult(io.NopCloser(strings.NewReader(tc.body))), nil)

			result, err := client.Query(t.Context(), "org1", "from(bucket: \"b\")")

			assert.Equal(t, tc.expResult, result)
			assert.Equal(t, tc.expErr, fmt.Sprint(err))
		})
	}
}

// fakeWriteAPIBlocking is a minimal api.WriteAPIBlocking implementation used to capture written points.
type fakeWriteAPIBlocking struct {
	api.WriteAPIBlocking
	err    error
	points []*write.Point
}

func (f *fakeWriteAPIBlocking) WritePoint(_ context.Context, point ...*write.Point) error {
	f.points = append(f.points, point...)

	return f.err
}

func Test_WritePoint(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		desc      string
		writeErr  error
		expErr    error
		expPoints int
	}{
		{desc: "successful write", writeErr: nil, expErr: nil, expPoints: 1},
		{desc: "write failure", writeErr: errWritePoint, expErr: errWritePoint, expPoints: 1},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := *setupDB(t, ctrl)
			mockClient := client.influx.client.(*Mockclient)
			writer := &fakeWriteAPIBlocking{err: tc.writeErr}

			mockClient.EXPECT().WriteAPIBlocking("org", "bucket").Return(writer)

			err := client.WritePoint(t.Context(), "org", "bucket", "cpu",
				map[string]string{"host": "a"}, map[string]any{"usage": 1.5}, time.Unix(10, 0))

			require.ErrorIs(t, err, tc.expErr)
			require.Len(t, writer.points, tc.expPoints)
			assert.Equal(t, "cpu", writer.points[0].Name())
		})
	}
}

func Test_BucketErrors(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		desc      string
		setupMock func(m *Mockbucket)
		call      func(ctx context.Context, c *Client) (any, error)
		expResult any
		expErr    error
	}{
		{
			desc: "create bucket failure",
			setupMock: func(m *Mockbucket) {
				m.EXPECT().CreateBucketWithNameWithID(gomock.Any(), "org", "b").Return(nil, errBucketOp)
			},
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.CreateBucket(ctx, "org", "b")
			},
			expResult: "",
			expErr:    errBucketOp,
		},
		{
			desc: "delete bucket failure",
			setupMock: func(m *Mockbucket) {
				m.EXPECT().DeleteBucketWithID(gomock.Any(), "id").Return(errBucketOp)
			},
			call: func(ctx context.Context, c *Client) (any, error) {
				return nil, c.DeleteBucket(ctx, "id")
			},
			expResult: nil,
			expErr:    errBucketOp,
		},
		{
			desc: "list buckets failure",
			setupMock: func(m *Mockbucket) {
				m.EXPECT().FindBucketsByOrgName(gomock.Any(), "org").Return(nil, errBucketOp)
			},
			call: func(ctx context.Context, c *Client) (any, error) {
				return c.ListBuckets(ctx, "org")
			},
			expResult: map[string]string(nil),
			expErr:    errBucketOp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := setupDB(t, ctrl)
			mockBucket := NewMockbucket(ctrl)
			client.influx.bucket = mockBucket

			tc.setupMock(mockBucket)

			res, err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expResult, res)
		})
	}
}

func Test_UseTracer(t *testing.T) {
	tests := []struct {
		desc      string
		tracer    any
		expTracer trace.Tracer
	}{
		{desc: "opencensus tracer is set", tracer: trace.DefaultTracer, expTracer: trace.DefaultTracer},
		{desc: "unsupported tracer is ignored", tracer: "invalid", expTracer: nil},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := New(Config{})

			client.UseTracer(tc.tracer)

			assert.Equal(t, tc.expTracer, client.tracer)
		})
	}
}

func Test_QueryWithTracer(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		desc   string
		query  string
		expErr error
	}{
		{desc: "query with span attributes", query: "from(bucket: \"b\")", expErr: errFailedQuery},
		{desc: "empty query uses method type", query: "", expErr: errFailedQuery},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client := setupDB(t, ctrl)
			client.UseTracer(trace.DefaultTracer)

			mockClient := client.influx.client.(*Mockclient)
			mockQueryAPI := NewMockquery(ctrl)

			mockClient.EXPECT().QueryAPI("org").Return(mockQueryAPI)
			mockQueryAPI.EXPECT().Query(gomock.Any(), tc.query).Return(nil, tc.expErr)

			res, err := client.Query(t.Context(), "org", tc.query)

			require.ErrorIs(t, err, tc.expErr)
			assert.Nil(t, res)
		})
	}
}

func Test_GetOperationType(t *testing.T) {
	tests := []struct {
		desc  string
		query string
		exp   string
	}{
		{desc: "empty query", query: "   ", exp: ""},
		{desc: "flux query", query: " from(bucket) |> range()", exp: "FROM(BUCKET)"},
		{desc: "operation name", query: "create-bucket", exp: "CREATE-BUCKET"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.exp, getOperationType(tc.query))
		})
	}
}

func Test_QueryLogPrettyPrint(t *testing.T) {
	tests := []struct {
		desc        string
		log         QueryLog
		expContains []string
	}{
		{
			desc: "log with args",
			log: QueryLog{Operation: "Query", Query: "from(bucket)\n   |> range()", Duration: 12,
				Keyspace: "ks", Args: []any{"org  1", 2}},
			expContains: []string{"Query", "INFL", "12", "ks", "from(bucket) |> range()", " [org 1, 2]"},
		},
		{
			desc:        "log without args",
			log:         QueryLog{Operation: "Ping", Query: "ping", Duration: 1},
			expContains: []string{"Ping", "INFL", "ping"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			for _, exp := range tc.expContains {
				assert.Contains(t, buf.String(), exp)
			}
		})
	}
}

func Test_Connect(t *testing.T) {
	tests := []struct {
		desc       string
		status     int
		body       string
		expLogs    int
		expErrLogs int
	}{
		{
			desc:   "successful connection",
			status: http.StatusOK,
			body: `{"name":"influxdb","message":"ready for queries and writes","status":"pass",` +
				`"checks":[],"version":"v2.7.0","commit":"abc"}`,
			expLogs:    2,
			expErrLogs: 0,
		},
		{
			desc:       "health check failure",
			status:     http.StatusServiceUnavailable,
			body:       `{"name":"influxdb","message":"not ready","status":"fail","checks":[]}`,
			expLogs:    1,
			expErrLogs: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockLogger.EXPECT().Logf(gomock.Any(), srv.URL).Times(tc.expLogs)
			mockLogger.EXPECT().Errorf("InfluxDB health check failed: %v", gomock.Any()).Times(tc.expErrLogs)
			mockLogger.EXPECT().Debug(gomock.Any())
			mockMetrics.EXPECT().NewHistogram("app_influxdb_stats", gomock.Any(), gomock.Any())
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_influxdb_stats", gomock.Any(),
				"url", srv.URL, "type", "HEALTH-CHECK")

			client := New(Config{URL: srv.URL, Token: "token"})
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			client.Connect()

			assert.NotNil(t, client.influx.client)
			assert.NotNil(t, client.influx.organization)
			assert.NotNil(t, client.influx.bucket)
			assert.NotNil(t, client.influx.query)
		})
	}
}

// fakeOrganizationsAPI records calls made through the organization wrapper.
type fakeOrganizationsAPI struct {
	api.OrganizationsAPI
	org  *domain.Organization
	orgs *[]domain.Organization
	err  error
}

func (f *fakeOrganizationsAPI) GetOrganizations(context.Context, ...api.PagingOption) (*[]domain.Organization, error) {
	return f.orgs, f.err
}

func (f *fakeOrganizationsAPI) FindOrganizationByName(_ context.Context, _ string) (*domain.Organization, error) {
	return f.org, f.err
}

func (f *fakeOrganizationsAPI) CreateOrganizationWithName(_ context.Context, _ string) (*domain.Organization, error) {
	return f.org, f.err
}

func (f *fakeOrganizationsAPI) DeleteOrganizationWithID(_ context.Context, _ string) error {
	return f.err
}

func Test_OrganizationAPIWrapper(t *testing.T) {
	name := "org"
	org := &domain.Organization{Name: name}
	orgs := &[]domain.Organization{*org}

	tests := []struct {
		desc   string
		fake   *fakeOrganizationsAPI
		expOrg *domain.Organization
		expAll *[]domain.Organization
		expErr error
	}{
		{desc: "success", fake: &fakeOrganizationsAPI{org: org, orgs: orgs}, expOrg: org, expAll: orgs},
		{desc: "failure", fake: &fakeOrganizationsAPI{err: errOrgOp}, expErr: errOrgOp},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			w := NewInfluxdbOrganizationAPI(tc.fake)

			all, err := w.GetOrganizations(t.Context())
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expAll, all)

			found, err := w.FindOrganizationByName(t.Context(), name)
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expOrg, found)

			created, err := w.CreateOrganizationWithName(t.Context(), name)
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expOrg, created)

			require.ErrorIs(t, w.DeleteOrganizationWithID(t.Context(), "id"), tc.expErr)
		})
	}
}

// fakeBucketsAPI records calls made through the bucket wrapper.
type fakeBucketsAPI struct {
	api.BucketsAPI
	bucket  *domain.Bucket
	buckets *[]domain.Bucket
	err     error
}

func (f *fakeBucketsAPI) GetBuckets(context.Context, ...api.PagingOption) (*[]domain.Bucket, error) {
	return f.buckets, f.err
}

func (f *fakeBucketsAPI) FindBucketsByOrgName(context.Context, string, ...api.PagingOption) (*[]domain.Bucket, error) {
	return f.buckets, f.err
}

func (f *fakeBucketsAPI) FindBucketByName(context.Context, string) (*domain.Bucket, error) {
	return f.bucket, f.err
}

func (f *fakeBucketsAPI) FindBucketByID(context.Context, string) (*domain.Bucket, error) {
	return f.bucket, f.err
}

func (f *fakeBucketsAPI) CreateBucket(context.Context, *domain.Bucket) (*domain.Bucket, error) {
	return f.bucket, f.err
}

func (f *fakeBucketsAPI) CreateBucketWithName(context.Context, *domain.Organization, string,
	...domain.RetentionRule) (*domain.Bucket, error) {
	return f.bucket, f.err
}

func (f *fakeBucketsAPI) CreateBucketWithNameWithID(context.Context, string, string,
	...domain.RetentionRule) (*domain.Bucket, error) {
	return f.bucket, f.err
}

func (f *fakeBucketsAPI) UpdateBucket(context.Context, *domain.Bucket) (*domain.Bucket, error) {
	return f.bucket, f.err
}

func (f *fakeBucketsAPI) DeleteBucketWithID(context.Context, string) error {
	return f.err
}

func Test_BucketAPIWrapper(t *testing.T) {
	bkt := &domain.Bucket{Name: "b"}
	bkts := &[]domain.Bucket{*bkt}

	tests := []struct {
		desc      string
		fake      *fakeBucketsAPI
		expBucket *domain.Bucket
		expAll    *[]domain.Bucket
		expErr    error
	}{
		{desc: "success", fake: &fakeBucketsAPI{bucket: bkt, buckets: bkts}, expBucket: bkt, expAll: bkts},
		{desc: "failure", fake: &fakeBucketsAPI{err: errBucketOp}, expErr: errBucketOp},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			w := NewInfluxdbBucketAPI(tc.fake)
			ctx := t.Context()

			singleCalls := []func() (*domain.Bucket, error){
				func() (*domain.Bucket, error) { return w.FindBucketByName(ctx, "b") },
				func() (*domain.Bucket, error) { return w.FindBucketByID(ctx, "id") },
				func() (*domain.Bucket, error) { return w.CreateBucket(ctx, bkt) },
				func() (*domain.Bucket, error) { return w.CreateBucketWithName(ctx, &domain.Organization{}, "b") },
				func() (*domain.Bucket, error) { return w.CreateBucketWithNameWithID(ctx, "org", "b") },
				func() (*domain.Bucket, error) { return w.UpdateBucket(ctx, bkt) },
			}

			for _, call := range singleCalls {
				res, err := call()
				require.ErrorIs(t, err, tc.expErr)
				assert.Equal(t, tc.expBucket, res)
			}

			all, err := w.GetBuckets(ctx)
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expAll, all)

			all, err = w.FindBucketsByOrgName(ctx, "org")
			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expAll, all)

			require.ErrorIs(t, w.DeleteBucketWithID(ctx, "id"), tc.expErr)
		})
	}
}
