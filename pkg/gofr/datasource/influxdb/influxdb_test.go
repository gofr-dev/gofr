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

// upstreamCall is one call recorded by the fake upstream APIs below.
type upstreamCall struct {
	method string
	args   []any
}

// wrapperCase is one wrapper method under test: call invokes it, and the wrapper must forward exactly one
// upstream call named method with args expArgs(ctx), passing back the upstream value (expRes) and error.
type wrapperCase[W any] struct {
	desc    string
	call    func(ctx context.Context, w W) (any, error)
	expArgs func(ctx context.Context) []any
	expRes  any
	method  string
}

// runWrapperCases runs every case against a fresh fake, once succeeding and once failing with baseErr.
func runWrapperCases[W any](t *testing.T, tests []wrapperCase[W], baseErr error,
	newWrapper func(fail bool) (W, *[]upstreamCall)) {
	t.Helper()

	modes := []struct {
		fail    bool
		baseErr error
		expErr  func(method string) error
	}{
		{fail: false, baseErr: nil, expErr: func(string) error { return nil }},
		{fail: true, baseErr: baseErr, expErr: func(method string) error { return fmt.Errorf("%w: %s", baseErr, method) }},
	}

	for _, tc := range tests {
		for _, mode := range modes {
			t.Run(fmt.Sprintf("%s/fail=%t", tc.desc, mode.fail), func(t *testing.T) {
				w, calls := newWrapper(mode.fail)
				ctx := t.Context()

				res, err := tc.call(ctx, w)

				require.Equal(t, []upstreamCall{{method: tc.method, args: tc.expArgs(ctx)}}, *calls)
				assert.Equal(t, tc.expRes, res)
				require.ErrorIs(t, err, mode.baseErr)
				assert.Equal(t, mode.expErr(tc.method), err)
			})
		}
	}
}

// fakeOrganizationsAPI records every upstream call made through the organization wrapper and
// returns a value (and, when fail is set, an error) that is distinct for each upstream method.
type fakeOrganizationsAPI struct {
	api.OrganizationsAPI
	fail  bool
	calls []upstreamCall
}

func (f *fakeOrganizationsAPI) record(method string, args ...any) error {
	f.calls = append(f.calls, upstreamCall{method: method, args: args})

	if f.fail {
		return fmt.Errorf("%w: %s", errOrgOp, method)
	}

	return nil
}

func (f *fakeOrganizationsAPI) GetOrganizations(ctx context.Context, opts ...api.PagingOption) (*[]domain.Organization, error) {
	err := f.record("GetOrganizations", ctx, len(opts))

	return &[]domain.Organization{{Name: "GetOrganizations"}}, err
}

func (f *fakeOrganizationsAPI) FindOrganizationByName(ctx context.Context, orgName string) (*domain.Organization, error) {
	err := f.record("FindOrganizationByName", ctx, orgName)

	return &domain.Organization{Name: "FindOrganizationByName"}, err
}

func (f *fakeOrganizationsAPI) CreateOrganizationWithName(ctx context.Context, orgName string) (*domain.Organization, error) {
	err := f.record("CreateOrganizationWithName", ctx, orgName)

	return &domain.Organization{Name: "CreateOrganizationWithName"}, err
}

func (f *fakeOrganizationsAPI) DeleteOrganizationWithID(ctx context.Context, orgID string) error {
	return f.record("DeleteOrganizationWithID", ctx, orgID)
}

func Test_OrganizationAPIWrapper(t *testing.T) {
	tests := []wrapperCase[organization]{
		{
			desc: "GetOrganizations",
			call: func(ctx context.Context, w organization) (any, error) {
				return w.GetOrganizations(ctx, api.PagingWithLimit(5), api.PagingWithOffset(1))
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, 2} },
			expRes:  &[]domain.Organization{{Name: "GetOrganizations"}},
			method:  "GetOrganizations",
		},
		{
			desc: "FindOrganizationByName",
			call: func(ctx context.Context, w organization) (any, error) {
				return w.FindOrganizationByName(ctx, "org-find")
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "org-find"} },
			expRes:  &domain.Organization{Name: "FindOrganizationByName"},
			method:  "FindOrganizationByName",
		},
		{
			desc: "CreateOrganizationWithName",
			call: func(ctx context.Context, w organization) (any, error) {
				return w.CreateOrganizationWithName(ctx, "org-create")
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "org-create"} },
			expRes:  &domain.Organization{Name: "CreateOrganizationWithName"},
			method:  "CreateOrganizationWithName",
		},
		{
			desc: "DeleteOrganizationWithID",
			call: func(ctx context.Context, w organization) (any, error) {
				return nil, w.DeleteOrganizationWithID(ctx, "org-id")
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "org-id"} },
			method:  "DeleteOrganizationWithID",
		},
	}

	runWrapperCases(t, tests, errOrgOp, func(fail bool) (organization, *[]upstreamCall) {
		fake := &fakeOrganizationsAPI{fail: fail}

		return NewInfluxdbOrganizationAPI(fake), &fake.calls
	})
}

// fakeBucketsAPI records every upstream call made through the bucket wrapper and returns a value
// (and, when fail is set, an error) that is distinct for each upstream method.
type fakeBucketsAPI struct {
	api.BucketsAPI
	fail  bool
	calls []upstreamCall
}

func (f *fakeBucketsAPI) record(method string, args ...any) error {
	f.calls = append(f.calls, upstreamCall{method: method, args: args})

	if f.fail {
		return fmt.Errorf("%w: %s", errBucketOp, method)
	}

	return nil
}

func (f *fakeBucketsAPI) GetBuckets(ctx context.Context, opts ...api.PagingOption) (*[]domain.Bucket, error) {
	err := f.record("GetBuckets", ctx, len(opts))

	return &[]domain.Bucket{{Name: "GetBuckets"}}, err
}

func (f *fakeBucketsAPI) FindBucketsByOrgName(ctx context.Context, orgName string, opts ...api.PagingOption) (*[]domain.Bucket, error) {
	err := f.record("FindBucketsByOrgName", ctx, orgName, len(opts))

	return &[]domain.Bucket{{Name: "FindBucketsByOrgName"}}, err
}

func (f *fakeBucketsAPI) FindBucketByName(ctx context.Context, bucketName string) (*domain.Bucket, error) {
	err := f.record("FindBucketByName", ctx, bucketName)

	return &domain.Bucket{Name: "FindBucketByName"}, err
}

func (f *fakeBucketsAPI) FindBucketByID(ctx context.Context, bucketID string) (*domain.Bucket, error) {
	err := f.record("FindBucketByID", ctx, bucketID)

	return &domain.Bucket{Name: "FindBucketByID"}, err
}

func (f *fakeBucketsAPI) CreateBucket(ctx context.Context, b *domain.Bucket) (*domain.Bucket, error) {
	err := f.record("CreateBucket", ctx, b)

	return &domain.Bucket{Name: "CreateBucket"}, err
}

func (f *fakeBucketsAPI) CreateBucketWithName(ctx context.Context, org *domain.Organization, bucketName string,
	rules ...domain.RetentionRule) (*domain.Bucket, error) {
	err := f.record("CreateBucketWithName", ctx, org, bucketName, rules)

	return &domain.Bucket{Name: "CreateBucketWithName"}, err
}

func (f *fakeBucketsAPI) CreateBucketWithNameWithID(ctx context.Context, orgID, bucketName string,
	rules ...domain.RetentionRule) (*domain.Bucket, error) {
	err := f.record("CreateBucketWithNameWithID", ctx, orgID, bucketName, rules)

	return &domain.Bucket{Name: "CreateBucketWithNameWithID"}, err
}

func (f *fakeBucketsAPI) UpdateBucket(ctx context.Context, b *domain.Bucket) (*domain.Bucket, error) {
	err := f.record("UpdateBucket", ctx, b)

	return &domain.Bucket{Name: "UpdateBucket"}, err
}

func (f *fakeBucketsAPI) DeleteBucketWithID(ctx context.Context, bucketID string) error {
	return f.record("DeleteBucketWithID", ctx, bucketID)
}

func Test_BucketAPIWrapper(t *testing.T) {
	in := &domain.Bucket{Name: "in-bucket"}
	org := &domain.Organization{Name: "in-org"}
	rules := []domain.RetentionRule{{EverySeconds: 3600}}

	tests := []wrapperCase[bucket]{
		{
			desc: "GetBuckets",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.GetBuckets(ctx, api.PagingWithLimit(5))
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, 1} },
			expRes:  &[]domain.Bucket{{Name: "GetBuckets"}},
			method:  "GetBuckets",
		},
		{
			desc: "FindBucketsByOrgName",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.FindBucketsByOrgName(ctx, "org-name", api.PagingWithLimit(5), api.PagingWithOffset(1))
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "org-name", 2} },
			expRes:  &[]domain.Bucket{{Name: "FindBucketsByOrgName"}},
			method:  "FindBucketsByOrgName",
		},
		{
			desc: "FindBucketByName",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.FindBucketByName(ctx, "bucket-name")
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "bucket-name"} },
			expRes:  &domain.Bucket{Name: "FindBucketByName"},
			method:  "FindBucketByName",
		},
		{
			desc: "FindBucketByID",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.FindBucketByID(ctx, "bucket-id")
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "bucket-id"} },
			expRes:  &domain.Bucket{Name: "FindBucketByID"},
			method:  "FindBucketByID",
		},
		{
			desc: "CreateBucket",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.CreateBucket(ctx, in)
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, in} },
			expRes:  &domain.Bucket{Name: "CreateBucket"},
			method:  "CreateBucket",
		},
		{
			desc: "CreateBucketWithName",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.CreateBucketWithName(ctx, org, "bucket-name", rules...)
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, org, "bucket-name", rules} },
			expRes:  &domain.Bucket{Name: "CreateBucketWithName"},
			method:  "CreateBucketWithName",
		},
		{
			desc: "CreateBucketWithNameWithID",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.CreateBucketWithNameWithID(ctx, "org-id", "bucket-name", rules...)
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "org-id", "bucket-name", rules} },
			expRes:  &domain.Bucket{Name: "CreateBucketWithNameWithID"},
			method:  "CreateBucketWithNameWithID",
		},
		{
			desc: "UpdateBucket",
			call: func(ctx context.Context, w bucket) (any, error) {
				return w.UpdateBucket(ctx, in)
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, in} },
			expRes:  &domain.Bucket{Name: "UpdateBucket"},
			method:  "UpdateBucket",
		},
		{
			desc: "DeleteBucketWithID",
			call: func(ctx context.Context, w bucket) (any, error) {
				return nil, w.DeleteBucketWithID(ctx, "bucket-id")
			},
			expArgs: func(ctx context.Context) []any { return []any{ctx, "bucket-id"} },
			method:  "DeleteBucketWithID",
		},
	}

	runWrapperCases(t, tests, errBucketOp, func(fail bool) (bucket, *[]upstreamCall) {
		fake := &fakeBucketsAPI{fail: fail}

		return NewInfluxdbBucketAPI(fake), &fake.calls
	})
}
