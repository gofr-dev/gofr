package datastore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/event"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

func TestGetNewMongoDB_ContextErr(t *testing.T) {
	mongoConfig := MongoConfig{"fake_host", "9999", "admin", "admin123", "test", false, false, 30}
	expErr := context.DeadlineExceeded

	_, err := GetNewMongoDB(log.NewLogger(), &mongoConfig)
	if err != nil && !strings.Contains(err.Error(), expErr.Error()) {
		t.Errorf("Error in testcase. Expected: %v, Got: %v", expErr, err)
	}
}

func TestGetNewMongoDB_ConnectionError(t *testing.T) {
	mongoConfig := MongoConfig{"", "", "", "", "test", false, false, 30}
	expErr := errors.New("error validating uri: username required if URI contains user info")

	_, err := GetNewMongoDB(log.NewLogger(), &mongoConfig)
	if err != nil && !strings.Contains(err.Error(), expErr.Error()) {
		t.Errorf("Error in testcase. Expected: %v, Got: %v", expErr, err)
	}
}

func TestGetMongoDBFromEnv_Success(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	_ = config.NewGoDotEnvProvider(logger, "../../configs")

	// Checking for connection with default env vars
	_, err := GetMongoDBFromEnv(logger)
	if err != nil {
		t.Error(err)
	}
}

func TestGetMongoDBFromEnv_Error(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	testcases := []struct {
		envKey    string
		newEnvVal string
		expErr    error
	}{
		{"MONGO_DB_HOST", "fake_host", context.DeadlineExceeded},
		{"MONGO_DB_ENABLE_SSL", "true", context.DeadlineExceeded},
		{"MONGO_DB_ENABLE_SSL", "non_bool", &strconv.NumError{
			Func: "ParseBool",
			Num:  "non_bool",
			Err:  errors.New("invalid syntax"),
		}},
	}

	for i := range testcases {
		oldEnvVal := c.Get(testcases[i].envKey)

		t.Setenv(testcases[i].envKey, testcases[i].newEnvVal)

		// Checking for connection with default env vars
		_, err := GetMongoDBFromEnv(logger)
		if err != nil && !strings.Contains(err.Error(), testcases[i].expErr.Error()) {
			t.Errorf("Expected %v but got %v", testcases[i].expErr, err)
		}

		t.Setenv(testcases[i].envKey, oldEnvVal)
	}
}

func TestGetMongoConfigFromEnv_SSL_RetryWrites(t *testing.T) {
	c := config.NewGoDotEnvProvider(log.NewMockLogger(io.Discard), "../../configs")
	oldEnableSSL := c.Get("MONGO_DB_ENABLE_SSL")
	oldretryWrites := c.Get("MONGO_DB_RETRY_WRITES")

	testcases := []struct {
		enableSSL   string
		retryWrites string
		expErr      error
	}{
		{"non_bool", "false", &strconv.NumError{
			Func: "ParseBool",
			Num:  "non_bool",
			Err:  errors.New("invalid syntax"),
		}},
		{"false", "non_bool", &strconv.NumError{
			Func: "ParseBool",
			Num:  "non_bool",
			Err:  errors.New("invalid syntax"),
		}},
	}

	for i := range testcases {
		t.Setenv("MONGO_DB_ENABLE_SSL", testcases[i].enableSSL)

		t.Setenv("MONGO_DB_RETRY_WRITES", testcases[i].retryWrites)

		_, err := getMongoConfigFromEnv()
		if !reflect.DeepEqual(err, testcases[i].expErr) {
			t.Errorf("Expected: %v, Got:%v", testcases[i].expErr, err)
		}
	}

	t.Setenv("MONGO_DB_ENABLE_SSL", oldEnableSSL)

	t.Setenv("MONGO_DB_RETRY_WRITES", oldretryWrites)
}

func Test_getMongoConnectionString(t *testing.T) {
	testcases := []struct {
		config        MongoConfig
		expConnString string
	}{
		{
			MongoConfig{"any_host", "9999", "admin", "admin123", "test", true, false, 30},
			"mongodb://admin:admin123@any_host:9999/?ssl=true&retrywrites=false",
		},
		{
			MongoConfig{"", "", "", "", "test", false, true, 30},
			"mongodb://:@:/?ssl=false&retrywrites=true",
		},
	}

	for i := range testcases {
		connStr := getMongoConnectionString(&testcases[i].config)
		if connStr != testcases[i].expConnString {
			t.Errorf("Testcase[%v] failed. Expected: %v, \nGot: %v", i, testcases[i].expConnString, connStr)
		}
	}
}

func TestDataStore_HealthCheck(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	tests := []struct {
		config   MongoConfig
		expected types.Health
	}{
		{MongoConfig{HostName: c.Get("MONGO_DB_HOST"), Port: c.Get("MONGO_DB_PORT"),
			Username: c.Get("MONGO_DB_USER"), Password: c.Get("MONGO_DB_PASS"), Database: c.Get("MONGO_DB_NAME"),
		}, types.Health{Name: "mongo", Status: "UP", Host: c.Get("MONGO_DB_HOST"), Database: "test"}},
		{MongoConfig{HostName: "random", Port: c.Get("MONGO_DB_PORT"), Username: c.Get("MONGO_DB_USER"),
			Password: c.Get("MONGO_DB_PASS"), Database: c.Get("MONGO_DB_NAME")},
			types.Health{Name: MongoStore, Status: pkg.StatusDown, Host: "random", Database: "test"},
		},
	}

	for i, tc := range tests {
		conn, _ := GetNewMongoDB(logger, &tc.config)

		output := conn.HealthCheck()
		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("TESTCASE [%v] FAILED, Got %v, Expected %v", i, output, tc.expected)
		}
	}
}

func TestDataStore_HealthCheck_Logs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	conf := &MongoConfig{HostName: c.Get("MONGO_DB_HOST"), Port: c.Get("MONGO_DB_PORT"),
		Username: c.Get("MONGO_DB_USER"), Password: c.Get("MONGO_DB_PASS"),
	}
	expectedResponse := types.Health{
		Name:     MongoStore,
		Status:   pkg.StatusDown,
		Host:     conf.HostName,
		Database: conf.Database,
	}

	m := mongodb{
		config: conf,
		logger: logger,
	}

	expectedLogMessage := "Health check failed for mongo Reason: MongoDB not initialized"

	resp := m.HealthCheck()

	assert.Contains(t, b.String(), expectedLogMessage, "TESTCASE FAILED. \nExpected: %v, \nGot: %v", expectedLogMessage, b.String())

	assert.Equal(t, expectedResponse, resp, "TESTCASE FAILED. \nExpected: %v, \nGot: %v", expectedResponse, resp)
}

// TestDataStore_HealthCheck_Down tests the health check response when db was connected but goes down
func TestDataStore_HealthCheck_Down(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	conf := &MongoConfig{HostName: c.Get("MONGO_DB_HOST"), Port: c.Get("MONGO_DB_PORT"),
		Username: c.Get("MONGO_DB_USER"), Password: c.Get("MONGO_DB_PASS"), Database: c.Get("MONGO_DB_NAME"),
	}
	expectedResponse := types.Health{
		Name:     MongoStore,
		Status:   pkg.StatusDown,
		Host:     conf.HostName,
		Database: conf.Database,
	}

	m := mongodb{
		Database: nil,
		config:   conf,
		logger:   logger,
	}

	resp := m.HealthCheck()
	if !reflect.DeepEqual(resp, expectedResponse) {
		t.Errorf("expected %v\tgot %v", expectedResponse, resp)
	}
}

func Test_MonitorMongo(t *testing.T) {
	testcases := []struct {
		query    string
		duration float64

		expectedLog string
	}{
		{"find", 100222.0, `{"datastore":"","duration":100,"host":"localhost","query":["find"]}`},
		{"insert customers", 213403, `{"datastore":"","duration":213,"host":"localhost","query":["insert customers"]}`},
		{`{"insert":"customers"}`, 213, `{"datastore":"","duration":0,"host":"localhost","query":["{\"insert\":\"customers\"}"]}`},
	}

	for i, tc := range testcases {
		q := QueryLogger{Hosts: "localhost", Query: []string{tc.query}}
		m := mongoMonitor{QueryLogger: &q}

		b := new(bytes.Buffer)
		m.Logger = log.NewMockLogger(b)

		m.monitorMongo(testcases[i].query, "", testcases[i].duration)

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE%d]Failed.Expected: %v\nGot: %v", i+1, tc.expectedLog, b.String())
		}
	}
}

func Test_MongoMonitorStarted(t *testing.T) {
	testcases := []struct {
		command   bson.D
		requestID int64

		expectedEvent map[int64]string
	}{
		{bson.D{primitive.E{Key: "find", Value: "customers"}}, 12030, map[int64]string{12030: `{"find":"customers"}`}},
		{bson.D{primitive.E{Key: "insert", Value: "customers"}}, 8923, map[int64]string{8923: `{"insert":"customers"}`}},
		{bson.D{primitive.E{Key: "insert", Value: "customers"}, {Key: "$db", Value: "test"}}, 8923,
			map[int64]string{8923: `{"insert":"customers"}`}},
	}

	for i, tc := range testcases {
		raw, _ := bson.Marshal(tc.command)

		m := mongoMonitor{event: &sync.Map{}}
		evt := event.CommandStartedEvent{Command: raw, RequestID: tc.requestID}

		m.Started(context.Background(), &evt)

		got := toMap(m.event)

		if !reflect.DeepEqual(got, tc.expectedEvent) {
			t.Errorf("[TESTCASE%d]Failed.Expected: %v\nGot: %v", i+1, tc.expectedEvent, got)
		}
	}
}

func Test_MongoMonitorSucceeded(t *testing.T) {
	testcases := []struct {
		event     map[int64]string
		duration  float64
		requestID int64

		expectedMap map[int64]string
		expectedLog string
	}{
		{map[int64]string{3452: `{"insert":"test"}`}, 12300, 3452, map[int64]string{},
			`{"datastore":"","duration":12,"host":"localhost","query":["{\"insert\":\"test\"}"]}`},
		{map[int64]string{2738: `{"insert":"test"}`, 89098: `{"insert":"customers"}`}, 78, 2738,
			map[int64]string{89098: `{"insert":"customers"}`},
			`{"datastore":"","duration":0,"host":"localhost","query":["{\"insert\":\"test\"}"]}`},
	}

	for i, tc := range testcases {
		q := QueryLogger{Hosts: "localhost"}
		m := mongoMonitor{QueryLogger: &q, event: toSyncMap(tc.event)}

		b := new(bytes.Buffer)
		m.Logger = log.NewMockLogger(b)

		m.Succeeded(context.Background(), &event.CommandSucceededEvent{CommandFinishedEvent: event.CommandFinishedEvent{
			DurationNanos: int64(tc.duration), RequestID: tc.requestID}})

		got := toMap(m.event)

		// ensuring the key is deleted
		if !reflect.DeepEqual(got, tc.expectedMap) {
			t.Errorf("[TESTCASE%d]Failed.Expected: %v\nGot: %v", i+1, tc.expectedMap, got)
		}

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE%d]Failed.Expected: %v\nGot: %v", i+1, tc.expectedLog, b.String())
		}
	}
}

func Test_MongoMonitorFailed(t *testing.T) {
	testcases := []struct {
		event     map[int64]string
		duration  float64
		requestID int64

		expectedMap map[int64]string
		expectedLog string
	}{
		{map[int64]string{92111: `{"delete":"test"}`}, 12300, 92111, map[int64]string{},
			`{"datastore":"","duration":12,"host":"localhost","query":["{\"delete\":\"test\"}"]}`},
		{map[int64]string{732383: `{"find":"test"}`, 89098: `{"insert":"customers}`}, 78,
			732383, map[int64]string{89098: `{"insert":"customers}`},
			`{"datastore":"","duration":0,"host":"localhost","query":["{\"find\":\"test\"}"]}`},
	}

	for i, tc := range testcases {
		q := QueryLogger{Hosts: "localhost"}
		m := mongoMonitor{QueryLogger: &q, event: toSyncMap(tc.event)}

		b := new(bytes.Buffer)
		m.Logger = log.NewMockLogger(b)

		m.Failed(context.Background(), &event.CommandFailedEvent{CommandFinishedEvent: event.CommandFinishedEvent{
			DurationNanos: int64(tc.duration), RequestID: tc.requestID}})

		got := toMap(m.event)

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE%d]Failed.Expected: %v\nGot: %v", i+1, tc.expectedLog, b.String())
		}

		// ensuring the key is deleted
		if !reflect.DeepEqual(got, tc.expectedMap) {
			t.Errorf("[TESTCASE%d]Failed.Expected: %v\nGot: %v", i+1, tc.expectedMap, got)
		}
	}
}

// toSyncMap converts map to sync Map to make testcases more readable
func toSyncMap(m map[int64]string) *sync.Map {
	var res sync.Map

	if m == nil {
		return nil
	}

	for key, val := range m {
		res.Store(key, val)
	}

	return &res
}

// toMap takes a sync Map and converts it to a map[int64]string for comparing got and expected,
// since sync maps cannot be compared
func toMap(m *sync.Map) map[int64]string {
	var res = make(map[int64]string)

	m.Range(func(key, value any) bool {
		k, _ := key.(int64)
		v, _ := value.(string)
		res[k] = v

		return true
	})

	return res
}
