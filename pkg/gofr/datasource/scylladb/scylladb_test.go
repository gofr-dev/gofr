package scylladb

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// samePointerMatcher matches only the exact pointer it was built with, so it
// can tell apart "the same destination reached the session" from "an equal
// looking value reached it" the way gomock.Eq's reflect.DeepEqual would.
type samePointerMatcher struct {
	want any
}

func (m samePointerMatcher) Matches(x any) bool { return x == m.want }
func (m samePointerMatcher) String() string     { return fmt.Sprintf("is exactly %v", m.want) }

func samePointer(want any) gomock.Matcher { return samePointerMatcher{want: want} }

const mockBatchName = "mockBatch"

var (
	errConnFail = errors.New("connection failed")
	errMock     = errors.New("test error")
)

type mockDependencies struct {
	mockSession *Mocksession
	mockQuery   *Mockquery
	mockBatch   *Mockbatch
	mockIter    *Mockiterator
	mockLogger  *MockLogger
	ctrl        *gomock.Controller
}

func initTest(t *testing.T) (*Client, *mockDependencies) {
	t.Helper()
	ctrl := gomock.NewController(t)

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockSession := NewMocksession(ctrl)
	mockQuery := NewMockquery(ctrl)
	mockBatch := NewMockbatch(ctrl)
	mockiter := NewMockiterator(ctrl)

	config := Config{
		Host:     "host1",
		Port:     9042,
		Keyspace: "my_Keyspace",
	}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.scylla.session = mockSession
	client.scylla.batches = map[string]batch{mockBatchName: mockBatch}

	mockMetrics.EXPECT().RecordHistogram(gomock.AssignableToTypeOf(context.Background()), "app_scylla_stats",
		gomock.AssignableToTypeOf(float64(0)), "hostname", client.config.Host, "keyspace", client.config.Keyspace).
		AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf("we did not get a pointer. data is not settable.").AnyTimes()
	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()

	return client, &mockDependencies{mockSession: mockSession, mockQuery: mockQuery, mockBatch: mockBatch,
		mockIter: mockiter, mockLogger: mockLogger, ctrl: ctrl}
}

func TestScyllaDB_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockClusterConfig := NewMockclusterConfig(ctrl)

	config := Config{
		Host:     "host1",
		Port:     9042,
		Keyspace: "my_keyspace",
	}
	scylladbBuckets := []float64{
		50, 75, 100, 125, 150, 200, 300, 500, 750, 1000, 2000, 3000, 5000, 7500, 10000, // 50µs-10ms
		25000, 50000, 100000, 250000, 500000, 1000000, 5000000, 10000000, 30000000, 60000000, 120000000, 180000000, // 25ms-3min
	}

	testCases := []struct {
		desc       string
		mockCall   func()
		expSession session
	}{
		{"Successful connection", func() {
			mockClusterConfig.EXPECT().createSession().Return(&scyllaSession{}, nil).Times(1)
			mockMetrics.EXPECT().NewHistogram("app_scylla_stats", "Response time of ScyllaDB queries in microseconds",
				scylladbBuckets).Times(1)
			mockLogger.EXPECT().Debugf("connecting to ScyllaDB at %v on port %v to keyspace %v", "host1", 9042, "my_keyspace")
			mockLogger.EXPECT().Logf("connected to '%s' keyspace at host '%s' and port '%d'", "my_keyspace", "host1", 9042)
		}, &scyllaSession{}},
		{
			"Connection failed", func() {
				mockLogger.EXPECT().Debugf("connecting to ScyllaDB at %v on port %v to keyspace %v", "host1", 9042, "my_keyspace")
				mockClusterConfig.EXPECT().createSession().Return(nil, errConnFail).Times(1)
				mockLogger.EXPECT().Error("failed to connect to ScyllaDB:")
			}, nil,
		},
	}
	for i, tc := range testCases {
		tc.mockCall()

		client := New(config)
		client.UseLogger(mockLogger)
		client.UseMetrics(mockMetrics)

		client.scylla.clusterConfig = mockClusterConfig
		client.Connect()
		assert.Equal(t, tc.expSession, client.scylla.session, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_Query(t *testing.T) {
	const query = "SELECT id, name FROM Users"

	type Users struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStructSlice := make([]Users, 0)
	mockIntSlice := make([]int, 0)
	mockStruct := Users{}
	mockInt := 0
	mockErrSlice := make([]int, 0)
	mockErrSliceAfterScan := make([]int, 0)

	client, mockDeps := initTest(t)

	testCases := []struct {
		desc     string
		dest     any
		mockCall func()
		expRes   any
		expErr   error
	}{
		{"success case: struct slice", &mockStructSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().Iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().NumRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().Columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockDeps.mockIter.EXPECT().Scan(gomock.Any()).Times(1)
			mockDeps.mockIter.EXPECT().Close().Return(nil).Times(1)
		}, &mockStructSlice, nil},
		{"success case: int slice", &mockIntSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().Iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().NumRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().Scan(gomock.Any()).Times(1)
			mockDeps.mockIter.EXPECT().Close().Return(nil).Times(1)
		}, &mockIntSlice, nil},
		{"success case: struct", &mockStruct, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().Iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().Columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockDeps.mockIter.EXPECT().Scan(gomock.Any()).Times(1)
			mockDeps.mockIter.EXPECT().Close().Return(nil).Times(1)
		}, &mockStruct, nil},
		{"failure case: dest is not pointer", mockStructSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug("we did not get a pointer. data is not settable.")
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
		}, mockStructSlice, errDestinationIsNotPointer},
		{"failure case: dest is int", &mockInt, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().Iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().Close().Return(nil).Times(1)
		}, &mockInt, errUnexpectedPointer{target: "int"}},
		{"failure case: query fails before any row is read", &mockErrSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().Iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().NumRows().Return(0).Times(1)
			mockDeps.mockIter.EXPECT().Close().Return(errMock).Times(1)
		}, &mockErrSlice, errMock},
		{"failure case: query fails after a row is read", &mockErrSliceAfterScan, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().Iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().NumRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().Scan(gomock.Any()).Times(1)
			mockDeps.mockIter.EXPECT().Close().Return(errMock).Times(1)
		}, &mockErrSliceAfterScan, errMock},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.Query(tc.dest, query)

		assert.Equalf(t, tc.expRes, tc.dest, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_Exec(t *testing.T) {
	const query = "INSERT INTO Users (id, name) VALUES(1, 'Test')"

	client, mockDeps := initTest(t)

	// Exec forwards its variadic values with `values...`, so a call with no values reaches
	// the session as Query(stmt) with nothing after it. Expecting Query(stmt, nil) makes
	// gomock look for a variadic argument that is never passed, and matches nothing.
	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"success case", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).AnyTimes()
			mockDeps.mockQuery.EXPECT().Exec().Return(nil).Times(1)
		}, nil},
		{"failure case", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).AnyTimes()
			mockDeps.mockQuery.EXPECT().Exec().Return(errMock).Times(1)
		}, errMock},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.Exec(query)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_NewBatch(t *testing.T) {
	const batchName = "testBatch"

	client, mockDeps := initTest(t)

	testCases := []struct {
		desc      string
		batchType int
		mockCall  func()
		expErr    error
	}{
		{"valid log type", LoggedBatch, func() {
			mockDeps.mockSession.EXPECT().newBatch(gocql.BatchType(LoggedBatch)).Return(&scyllaBatch{}).Times(1)
		}, nil},
		{"valid log type, empty batches", LoggedBatch, func() {
			client.scylla.batches = nil

			mockDeps.mockSession.EXPECT().newBatch(gocql.BatchType(LoggedBatch)).Return(&scyllaBatch{}).Times(1)
		}, nil},
		{"invalid log type", -1, func() {}, errUnsupportedBatchType},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.NewBatch(batchName, tc.batchType)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)

		if tc.expErr != nil {
			_, ok := client.scylla.batches[batchName]

			assert.Truef(t, ok, "TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}

func Test_HealthCheck(t *testing.T) {
	const query = "SELECT now() FROM system.local"

	client, mockDeps := initTest(t)

	testCases := []struct {
		desc      string
		mockCall  func()
		expHealth *Health
		err       error
	}{
		{"success case", func() {
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().ExecWithCtx(gomock.Any()).Return(nil).Times(1)
		}, &Health{
			Status:  "UP",
			Details: map[string]any{"host": client.config.Host, "keyspace": client.config.Keyspace},
		}, nil},
		{"failure case: exec error", func() {
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().ExecWithCtx(gomock.Any()).Return(errMock).Times(1)
		}, &Health{
			Status: "DOWN",
			Details: map[string]any{"host": client.config.Host, "keyspace": client.config.Keyspace,
				"message": errMock.Error()},
		}, errStatusDown},
		{"failure case: ScyllaDB not initialized", func() {
			client.scylla.session = nil
		}, &Health{
			Status: "DOWN",
			Details: map[string]any{"host": client.config.Host, "keyspace": client.config.Keyspace,
				"message": "ScyllaDB not connected"},
		}, errStatusDown},
	}

	for i, tc := range testCases {
		tc.mockCall()

		health, err := client.HealthCheck(context.Background())

		assert.Equal(t, tc.err, err)
		assert.Equalf(t, tc.expHealth, health, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_BatchQuery(t *testing.T) {
	client, mockDeps := initTest(t)

	const stmt = "INSERT INTO users (id, name) VALUES(?, ?)"

	values := []any{1, "Test"}

	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"batch is initialized", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockBatch.EXPECT().Query(stmt, values...)
		}, nil},
		{"batch is not initialized", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))

			client.scylla.batches = nil
		}, errBatchNotInitialized},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.BatchQuery(mockBatchName, stmt, values...)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExecuteBatchCAS(t *testing.T) {
	client, mockDeps := initTest(t)

	type testStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStructSlice := make([]testStruct, 0)

	testCases := []struct {
		desc     string
		dest     any
		mockCall func()
		expRes   any
		expErr   error
	}{
		{"success case: struct slice", &mockStructSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().executeBatchCAS(mockDeps.mockBatch, gomock.Any()).Return(true, nil).Times(1)
		}, &mockStructSlice, nil},
		{"failure case: executeBatchCAS returns error", &mockStructSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().executeBatchCAS(mockDeps.mockBatch, gomock.Any()).Return(false, assert.AnError).Times(1)
		}, &mockStructSlice, assert.AnError},
		{"failure case: batch not initialized", &mockStructSlice, func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))

			client.scylla.batches = nil
		}, &mockStructSlice, errBatchNotInitialized},
	}

	for i, tc := range testCases {
		tc.mockCall()

		applied, err := client.ExecuteBatchCAS(mockBatchName, tc.dest)

		assert.Equalf(t, tc.expRes, tc.dest, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, applied, tc.expErr == nil, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExecuteBatchCASForwardsDest(t *testing.T) {
	client, mockDeps := initTest(t)

	var (
		id   int
		name string
	)

	mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
	mockDeps.mockSession.EXPECT().
		executeBatchCAS(mockDeps.mockBatch, samePointer(&id), samePointer(&name)).
		Return(true, nil).Times(1)

	applied, err := client.ExecuteBatchCAS(mockBatchName, &id, &name)

	require.NoError(t, err)
	assert.True(t, applied)
}

func Test_ExecCAS(t *testing.T) {
	const query = "INSERT INTO users (id, name) VALUES(1, 'Test') IF NOT EXISTS"

	type users struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStruct := users{}
	mockInt := 0

	// Each row gets its own client and mocks (initTest creates a fresh gomock
	// controller per call) so one row's expectations can never be satisfied
	// by another row's leftover AnyTimes() stub.
	testCases := []struct {
		desc       string
		dest       any
		mockCall   func(mockDeps *mockDependencies)
		expApplied bool
		expErr     error
	}{
		{"success case: struct dest, applied true", &mockStruct, func(mockDeps *mockDependencies) {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery)
			mockDeps.mockQuery.EXPECT().MapScanCAS(gomock.AssignableToTypeOf(map[string]any{})).Return(true, nil)
		}, true, nil},

		{"success case: int dest, applied true", &mockInt, func(mockDeps *mockDependencies) {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery)
			mockDeps.mockQuery.EXPECT().ScanCAS(&mockInt).Return(true, nil)
		}, true, nil},

		{"failure case: struct dest, error", &mockStruct, func(mockDeps *mockDependencies) {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery)
			mockDeps.mockQuery.EXPECT().MapScanCAS(gomock.AssignableToTypeOf(map[string]any{})).Return(false, errMock)
		}, false, errMock},
		{"failure case: dest is not pointer", mockInt, func(mockDeps *mockDependencies) {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
		}, false, errDestinationIsNotPointer},
		{"failure case: dest is slice", &[]int{}, func(mockDeps *mockDependencies) {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery)
		}, false, errUnexpectedSlice{target: "[]*[]int"}},
		{"failure case: dest is map", &map[string]any{}, func(mockDeps *mockDependencies) {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(query).Return(mockDeps.mockQuery)
		}, false, errUnexpectedMap},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			client, mockDeps := initTest(t)

			tc.mockCall(mockDeps)

			applied, err := client.ExecCAS(tc.dest, query)

			assert.Equal(t, tc.expApplied, applied)
			assert.Equal(t, tc.expErr, err)
		})
	}
}

func Test_ExecCASForwardsValuesAndDest(t *testing.T) {
	const stmt = "UPDATE users SET name = ? WHERE id = ? IF name = ?"

	values := []any{"b", 1, "a"}

	testCases := []struct {
		desc       string
		scanErr    error
		expApplied bool
		expErr     error
		expName    string
	}{
		{desc: "applied", expApplied: true, expName: ""},
		{desc: "not applied, existing value scanned", expApplied: false, expName: "current"},
		{desc: "error", scanErr: errMock, expApplied: false, expErr: errMock},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			client, mockDeps := initTest(t)

			var name string

			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().Query(stmt, values...).Return(mockDeps.mockQuery)
			mockDeps.mockQuery.EXPECT().ScanCAS(&name).DoAndReturn(func(dest ...any) (bool, error) {
				*(dest[0].(*string)) = tc.expName

				return tc.expApplied, tc.scanErr
			})

			applied, err := client.ExecCAS(&name, stmt, values...)

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expApplied, applied)
			assert.Equal(t, tc.expName, name)
		})
	}
}

func TestClient_ExecuteBatchCASWithCtx(t *testing.T) {
	client, mockDeps := initTest(t)

	type testStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStructSlice := make([]testStruct, 0)

	testCases := []struct {
		desc       string
		batchName  string
		dest       any
		mockCall   func()
		expApplied bool
		expErr     error
	}{
		{
			desc:      "success case: batch found and executed",
			batchName: mockBatchName,
			dest:      &mockStructSlice,
			mockCall: func() {
				mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
				mockDeps.mockSession.EXPECT().executeBatchCAS(mockDeps.mockBatch, gomock.Any()).Return(true, nil).Times(1)
			},
			expApplied: true,
			expErr:     nil,
		},
		{
			desc:      "failure case: executeBatchCAS returns error",
			batchName: mockBatchName,
			dest:      &mockStructSlice,
			mockCall: func() {
				mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
				mockDeps.mockSession.EXPECT().executeBatchCAS(mockDeps.mockBatch, gomock.Any()).Return(false, assert.AnError).Times(1)
			},
			expApplied: false,
			expErr:     assert.AnError,
		},
		{
			desc:      "failure case: batch not initialized",
			batchName: "non-existent-batch",
			dest:      &mockStructSlice,
			mockCall: func() {
				mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))

				client.scylla.batches = nil
			},
			expApplied: false,
			expErr:     errBatchNotInitialized,
		},
	}

	for i, tc := range testCases {
		tc.mockCall()

		ctx := context.Background()
		applied, err := client.ExecuteBatchCASWithCtx(ctx, tc.batchName, tc.dest)

		assert.Equalf(t, tc.expApplied, applied, "TEST[%d], Failed: %s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed: %s", i, tc.desc)
	}
}
func Test_ExecuteBatch(t *testing.T) {
	client, mockDeps := initTest(t)

	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"execute batch success", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().executeBatch(mockDeps.mockBatch).Return(nil).Times(1)
		}, nil},
		{"execute batch failure", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))
			mockDeps.mockSession.EXPECT().executeBatch(mockDeps.mockBatch).Return(errMock).Times(1)
		}, errMock},
		{"batch not initialized", func() {
			mockDeps.mockLogger.EXPECT().Debug(gomock.AssignableToTypeOf(&QueryLog{}))

			client.scylla.batches = nil
		}, errBatchNotInitialized},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.ExecuteBatch(mockBatchName)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

// Test_HealthCheck_PropagatesContext asserts the context passed to HealthCheck is
// forwarded to the health-check query, so a caller's deadline/cancellation applies.
func Test_HealthCheck_PropagatesContext(t *testing.T) {
	client, mockDeps := initTest(t)

	type ctxKey string

	const key ctxKey = "health-check"

	ctx := context.WithValue(t.Context(), key, "yes")

	mockDeps.mockSession.EXPECT().Query("SELECT now() FROM system.local").Return(mockDeps.mockQuery).Times(1)
	mockDeps.mockQuery.EXPECT().
		ExecWithCtx(gomock.Cond(func(x any) bool {
			c, ok := x.(context.Context)
			return ok && c.Value(key) == "yes"
		})).
		Return(nil).Times(1)

	_, err := client.HealthCheck(ctx)

	require.NoError(t, err)
	require.True(t, mockDeps.ctrl.Satisfied(), "health-check query was not executed")
}
