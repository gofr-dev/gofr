package cassandra

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

var (
	errConnFail = errors.New("connection failure")
	errMock     = errors.New("test error")
)

type mockDependencies struct {
	mockSession *Mocksession
	mockQuery   *Mockquery
	mockBatch   *Mockbatch
	mockIter    *Mockiterator
}

func initTest(t *testing.T) (*Client, *Batch, *mockDependencies) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := new(bytes.Buffer)
	mockLogger := NewMockLogger(INFO, b)
	mockMetrics := NewMockMetrics(ctrl)
	mockSession := NewMocksession(ctrl)
	mockQuery := NewMockquery(ctrl)
	mockBatch := NewMockbatch(ctrl)
	mockIter := NewMockiterator(ctrl)

	config := Config{
		Hosts:    "host1",
		Port:     9042,
		Keyspace: "test_keyspace",
	}

	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)

	client.cassandra.session = mockSession

	batchClient := &Batch{Client: client, batch: mockBatch}

	mockMetrics.EXPECT().RecordHistogram(gomock.AssignableToTypeOf(context.Background()), "app_cassandra_stats",
		gomock.AssignableToTypeOf(float64(0)), "hostname", client.config.Hosts, "keyspace", client.config.Keyspace).AnyTimes()

	return client, batchClient, &mockDependencies{mockSession: mockSession, mockQuery: mockQuery, mockBatch: mockBatch, mockIter: mockIter}
}

func Test_Connect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := new(bytes.Buffer)
	mockLogger := NewMockLogger(INFO, b)
	mockMetrics := NewMockMetrics(ctrl)
	mockclusterConfig := NewMockclusterConfig(ctrl)

	config := Config{
		Hosts:    "host1",
		Port:     9042,
		Keyspace: "test_keyspace",
	}

	cassandraBucktes := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}

	testCases := []struct {
		desc       string
		mockCall   func()
		expLog     string
		expSession session
	}{
		{"successful connection", func() {
			mockclusterConfig.EXPECT().createSession().Return(&cassandraSession{}, nil).Times(1)
			mockMetrics.EXPECT().NewHistogram("app_cassandra_stats", "Response time of CASSANDRA queries in milliseconds.",
				cassandraBucktes).Times(1)
		}, "connected to 'test_keyspace' keyspace at host 'host1' and port '9042'", &cassandraSession{}},
		{"connection failure", func() {
			mockclusterConfig.EXPECT().createSession().Return(nil, errConnFail).Times(1)
		}, "error connecting to cassandra: connection failure", nil},
	}

	for i, tc := range testCases {
		tc.mockCall()

		client := New(config)
		client.UseLogger(mockLogger)
		client.UseMetrics(mockMetrics)

		client.cassandra.clusterConfig = mockclusterConfig

		client.Connect()

		assert.Equalf(t, tc.expSession, client.cassandra.session, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Containsf(t, b.String(), tc.expLog, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_Query(t *testing.T) {
	const query = "SELECT id, name FROM users"

	type users struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStructSlice := make([]users, 0)
	mockIntSlice := make([]int, 0)
	mockStruct := users{}
	mockInt := 0

	client, _, mockDeps := initTest(t)

	testCases := []struct {
		desc     string
		dest     interface{}
		mockCall func()
		expRes   interface{}
		expErr   error
	}{
		{"success case: struct slice", &mockStructSlice, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().numRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockDeps.mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockStructSlice, nil},
		{"success case: int slice", &mockIntSlice, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().numRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockIntSlice, nil},
		{"success case: struct", &mockStruct, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockDeps.mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockStruct, nil},
		{"failure case: dest is not pointer", mockStructSlice, func() {}, mockStructSlice,
			ErrDestinationIsNotPointer},
		{"failure case: dest is int", &mockInt, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().iter().Return(mockDeps.mockIter).Times(1)
		}, &mockInt, UnexpectedPointer{target: "int"}},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.Query(tc.dest, query)

		assert.Equalf(t, tc.expRes, tc.dest, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_Exec(t *testing.T) {
	const query = "INSERT INTO users (id, name) VALUES(1, 'Test')"

	client, _, mockDeps := initTest(t)

	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"success case", func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().exec().Return(nil).Times(1)
		}, nil},
		{"failure case", func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().exec().Return(errMock).Times(1)
		}, errMock},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.Exec(query)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExecCAS(t *testing.T) {
	const query = "INSERT INTO users (id, name) VALUES(1, 'Test') IF NOT EXISTS"

	type users struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStruct := users{}
	mockInt := 0

	client, _, mockDeps := initTest(t)

	testCases := []struct {
		desc       string
		dest       interface{}
		mockCall   func()
		expApplied bool
		expErr     error
	}{
		{"success case: struct dest, applied true", &mockStruct, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().mapScanCAS(gomock.AssignableToTypeOf(map[string]interface{}{})).Return(true, nil).Times(1)
		}, true, nil},
		{"success case: int dest, applied true", &mockInt, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().scanCAS(gomock.Any()).Return(true, nil).Times(1)
		}, true, nil},
		{"failure case: struct dest, error", &mockStruct, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().mapScanCAS(gomock.AssignableToTypeOf(map[string]interface{}{})).Return(false, errMock).Times(1)
		}, false, errMock},
		{"failure case: int dest, error", &mockInt, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().scanCAS(gomock.Any()).Return(false, errMock).Times(1)
		}, false, errMock},
		{"failure case: dest is not pointer", mockInt, func() {}, false, ErrDestinationIsNotPointer},
		{"failure case: dest is slice", &[]int{}, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
		}, false, UnexpectedSlice{target: "[]*[]int"}},
		{"failure case: dest is map", &map[string]interface{}{}, func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
		}, false, ErrUnexpectedMap},
	}

	for i, tc := range testCases {
		tc.mockCall()

		applied, err := client.ExecCAS(tc.dest, query)

		assert.Equalf(t, tc.expApplied, applied, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_HealthCheck(t *testing.T) {
	const query = "SELECT now() FROM system.local"

	client, _, mockDeps := initTest(t)

	testCases := []struct {
		desc      string
		mockCall  func()
		expHealth *Health
		err       error
	}{
		{"success case", func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().exec().Return(nil).Times(1)
		}, &Health{
			Status:  "UP",
			Details: map[string]interface{}{"host": client.config.Hosts, "keyspace": client.config.Keyspace},
		}, nil},
		{"failure case: exec error", func() {
			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().exec().Return(errMock).Times(1)
		}, &Health{
			Status: "DOWN",
			Details: map[string]interface{}{"host": client.config.Hosts, "keyspace": client.config.Keyspace,
				"message": errMock.Error()},
		}, errStatusDown},
		{"failure case: cassandra not initializes", func() {
			client.cassandra.session = nil

			mockDeps.mockSession.EXPECT().query(query).Return(mockDeps.mockQuery).Times(1)
			mockDeps.mockQuery.EXPECT().exec().Return(nil).Times(1)
		}, &Health{
			Status: "DOWN",
			Details: map[string]interface{}{"host": client.config.Hosts, "keyspace": client.config.Keyspace,
				"message": "cassandra not connected"},
		}, errStatusDown},
	}

	for i, tc := range testCases {
		tc.mockCall()

		health, err := client.HealthCheck(context.Background())

		assert.Equal(t, tc.err, err)
		assert.Equalf(t, tc.expHealth, health, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_CreateSession_Error(t *testing.T) {
	c := newClusterConfig(&Config{})

	sess, err := c.createSession()

	assert.Nil(t, sess, "Test Failed: should return error without creating session")
	assert.Error(t, err, "Test Failed: should return error without creating session")
}

func Test_cassandraSession_Query(t *testing.T) {
	c := &cassandraSession{session: &gocql.Session{}}

	q := c.query("sample query")

	assert.NotNil(t, q, "Test Failed")
	assert.IsType(t, &cassandraQuery{}, q, "Test Failed")
}
