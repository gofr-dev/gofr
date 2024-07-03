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

func initTest(t *testing.T) (*Client, *Mocksession, *Mockquery, *Mockiterator) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := new(bytes.Buffer)
	mockLogger := NewMockLogger(INFO, b)
	mockMetrics := NewMockMetrics(ctrl)
	mockSession := NewMocksession(ctrl)
	mockQuery := NewMockquery(ctrl)
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

	mockMetrics.EXPECT().RecordHistogram(gomock.AssignableToTypeOf(context.Background()), "app_cassandra_stats",
		gomock.AssignableToTypeOf(float64(0)), "hostname", client.config.Hosts, "keyspace", client.config.Keyspace).AnyTimes()

	return client, mockSession, mockQuery, mockIter
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

	client, mockSession, mockQuery, mockIter := initTest(t)

	testCases := []struct {
		desc     string
		dest     interface{}
		mockCall func()
		expRes   interface{}
		expErr   error
	}{
		{"success case: struct slice", &mockStructSlice, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().iter().Return(mockIter).Times(1)
			mockIter.EXPECT().numRows().Return(1).Times(1)
			mockIter.EXPECT().columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockStructSlice, nil},
		{"success case: int slice", &mockIntSlice, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().iter().Return(mockIter).Times(1)
			mockIter.EXPECT().numRows().Return(1).Times(1)
			mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockIntSlice, nil},
		{"success case: struct", &mockStruct, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().iter().Return(mockIter).Times(1)
			mockIter.EXPECT().columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockStruct, nil},
		{"failure case: dest is not pointer", mockStructSlice, func() {}, mockStructSlice,
			destinationIsNotPointer{}},
		{"failure case: dest is int", &mockInt, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().iter().Return(mockIter).Times(1)
		}, &mockInt, unexpectedPointer{target: "int"}},
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

	client, mockSession, mockQuery, _ := initTest(t)

	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"success case", func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().exec().Return(nil).Times(1)
		}, nil},
		{"failure case", func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().exec().Return(errMock).Times(1)
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

	client, mockSession, mockQuery, _ := initTest(t)

	testCases := []struct {
		desc       string
		dest       interface{}
		mockCall   func()
		expApplied bool
		expErr     error
	}{
		{"success case: struct dest, applied true", &mockStruct, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().mapScanCAS(gomock.AssignableToTypeOf(map[string]interface{}{})).Return(true, nil).Times(1)
		}, true, nil},
		{"success case: int dest, applied true", &mockInt, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().scanCAS(gomock.Any()).Return(true, nil).Times(1)
		}, true, nil},
		{"failure case: struct dest, error", &mockStruct, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().mapScanCAS(gomock.AssignableToTypeOf(map[string]interface{}{})).Return(false, errMock).Times(1)
		}, false, errMock},
		{"failure case: int dest, error", &mockInt, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().scanCAS(gomock.Any()).Return(false, errMock).Times(1)
		}, false, errMock},
		{"failure case: dest is not pointer", mockInt, func() {}, false, destinationIsNotPointer{}},
		{"failure case: dest is slice", &[]int{}, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
		}, false, unexpectedSlice{target: "[]*[]int"}},
		{"failure case: dest is map", &map[string]interface{}{}, func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
		}, false, unexpectedMap{}},
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

	client, mockSession, mockQuery, _ := initTest(t)

	testCases := []struct {
		desc      string
		mockCall  func()
		expHealth *Health
	}{
		{"success case", func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().exec().Return(nil).Times(1)
		}, &Health{
			Status:  "UP",
			Details: map[string]interface{}{"host": client.config.Hosts, "keyspace": client.config.Keyspace},
		}},
		{"failure case: exec error", func() {
			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().exec().Return(errMock).Times(1)
		}, &Health{
			Status: "DOWN",
			Details: map[string]interface{}{"host": client.config.Hosts, "keyspace": client.config.Keyspace,
				"message": errMock.Error()},
		}},
		{"failure case: cassandra not initializes", func() {
			client.cassandra.session = nil

			mockSession.EXPECT().query(query).Return(mockQuery).Times(1)
			mockQuery.EXPECT().exec().Return(nil).Times(1)
		}, &Health{
			Status: "DOWN",
			Details: map[string]interface{}{"host": client.config.Hosts, "keyspace": client.config.Keyspace,
				"message": "cassandra not connected"},
		}},
	}

	for i, tc := range testCases {
		tc.mockCall()

		health := client.HealthCheck()

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
