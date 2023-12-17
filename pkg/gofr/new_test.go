package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/datastore/pubsub/kafka"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

type mockConfig struct {
	testCase string
}

func (m mockConfig) Get(key string) string {
	switch m.testCase {
	case "redis error", "db error":
		return "mock"
	case "kafka error":
		if key == "KAFKA_HOSTS" {
			return ""
		}
	case "kafka":
		if key == "PUBSUB_BACKEND" {
			return "KAFKA"
		}
	case "avro":
		if key == "AVRO_SCHEMA_URL" {
			return "http://localhost:8081"
		}

		return ""
	case "avroerr":
		return ""
	default:
		c := &config.GoDotEnvProvider{}
		return c.Get(key)
	}

	return "mock"
}

func (m mockConfig) GetOrDefault(_, d string) string {
	return d
}

func Test_initializeDynamoDB(t *testing.T) {
	tcs := []struct {
		config Config
		output string
	}{
		{
			&config.MockConfig{Data: map[string]string{
				"DYNAMODB_ACCESS_KEY_ID":     "access-key-id",
				"DYNAMODB_SECRET_ACCESS_KEY": "access-key",
				"DYNAMODB_REGION":            "",
				"DYNAMODB_ENDPOINT_URL":      "",
				"DYNAMODB_CONN_RETRY":        "2",
			}},
			"DynamoDB could not be initialized",
		},
		{
			config.NewGoDotEnvProvider(log.NewMockLogger(io.Discard), "../../configs"),
			"DynamoDB initialized",
		},
	}

	for _, tc := range tcs {
		g := NewWithConfig(tc.config)
		b := new(bytes.Buffer)

		g.Logger = log.NewMockLogger(b)
		initializeDynamoDB(tc.config, g)

		if !strings.Contains(b.String(), tc.output) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tc.output, b.String())
		}
	}
}

func Test_initializeDynamoDB_EmptyLog(t *testing.T) {
	g := New()
	b := new(bytes.Buffer)

	g.Logger = log.NewMockLogger(b)
	initializeDynamoDB(&config.MockConfig{Data: map[string]string{}}, g)

	if strings.Contains(strings.ToLower(b.String()), "dynamodb") {
		t.Errorf("FAILED, did not expect DynamoDB in logs")
	}
}

func Test_initializeRedis(t *testing.T) {
	tcs := []struct {
		c      Config
		expStr string // expected in the logs, logged by g.Logger
	}{
		{mockConfig{testCase: "redis error"}, "could not connect to Redis"},
		{mockConfig{}, "Redis connected"},
	}

	for _, tc := range tcs {
		g := New()
		b := new(bytes.Buffer)

		g.Logger = log.NewMockLogger(b)
		initializeRedis(tc.c, g)

		if !strings.Contains(b.String(), tc.expStr) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tc.expStr, b.String())
		}
	}
}

func Test_RedisDBConnection(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	hostName := c.Get("REDIS_HOST")
	port := c.Get("REDIS_PORT")

	tcs := []struct {
		redisDB     string
		exp         int64
		Description string
	}{
		{"10", 1, "connect to redis 10 db and get count"},
		{"10", 2, "connect to redis 10 db check count increment"},
		{"9", 1, "connect to redis 10 db and get count"},
	}

	for i, tc := range tcs {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		mockConfig := config.MockConfig{
			Data: map[string]string{"REDIS_HOST": hostName,
				"REDIS_PORT": port,
				"REDIS_DB":   tc.redisDB,
			},
		}

		g := NewWithConfig(&mockConfig)

		g.Logger = logger

		inc, _ := g.Redis.Incr(context.Background(), "get-redis-db-connection").Result()
		assert.Equal(t, tc.exp, inc, "TEST[%d], failed.Expected:%v,Got:%v", i, tc.exp, inc)
	}
}

func Test_initializeDB(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	c := config.NewGoDotEnvProvider(logger, "../../configs")

	hostName := c.Get("DB_HOST")
	port := c.Get("DB_PORT")

	testcases := []struct {
		host        string
		port        string
		ORM         string
		expectedLog string
	}{
		{"", "", "", ""},
		{"incorrect-url", "7", "", "could not connect to DB"},
		{hostName, port, "", "DB connected, HostName: " + hostName + ", Port: " + port},
		{"incorrect-url", "7", "SQLX", "could not connect to DB"},
		{hostName, port, "SQLX", "DB connected, HostName: " + hostName + ", Port: " + port},
	}

	for i, tc := range testcases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		mockConfig := config.MockConfig{
			Data: map[string]string{"DB_HOST": tc.host, "DB_USER": c.Get("DB_USER"), "DB_PASSWORD": c.Get("DB_PASSWORD"),
				"DB_NAME": c.Get("DB_NAME"), "DB_PORT": tc.port, "DB_DIALECT": c.Get("DB_DIALECT"), "DB_ORM": tc.ORM,
				"DB_MAX_OPEN_CONN": c.Get("DB_MAX_OPEN_CONN"), "DB_MAX_IDLE_CONN": c.Get("DB_MAX_IDLE_CONN"),
				"DB_MAX_CONN_LIFETIME": c.Get("DB_MAX_CONN_LIFETIME"),
			},
		}

		g := NewWithConfig(&mockConfig)
		g.Logger = logger

		initializeDB(&mockConfig, g)

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE %d] Failed. Got: %v\tExpected: %v\n", i+1, b.String(), tc.expectedLog)
		}
	}
}

func Test_InitializeElasticsearch(t *testing.T) {
	testcases := []struct {
		config      Config
		expectedLog string
	}{
		{&config.MockConfig{Data: map[string]string{"ELASTIC_SEARCH_HOST": "", "ELASTIC_SEARCH_PORT": "",
			"ELASTIC_CLOUD_ID": ""}}, ""},
		{&config.MockConfig{Data: map[string]string{"ELASTIC_SEARCH_HOST": "localhost",
			"ELASTIC_SEARCH_PORT": "2012"}}, "connected to elasticsearch"},
		{&config.MockConfig{Data: map[string]string{"ELASTIC_SEARCH_HOST": "localhost",
			"ELASTIC_SEARCH_PORT": "2012", "ELASTIC_CLOUD_ID": "elastic-cloud-id"}},
			"could not connect to elasticsearch"},
	}

	for i, tc := range testcases {
		b := new(bytes.Buffer)

		g := NewWithConfig(tc.config)
		g.Logger = log.NewMockLogger(b)

		initializeElasticsearch(tc.config, g)

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE%v] Failed.\nExpected: %v\nGot: %v", i+1, tc.expectedLog, b.String())
		}
	}
}

func Test_initializeMongoDB(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	config.NewGoDotEnvProvider(logger, "../../configs")

	tcs := []struct {
		configLoc   Config
		expectedStr string
	}{
		{mockConfig{}, "MongoDB connected."},
		{configLoc: &config.MockConfig{Data: map[string]string{"MONGO_DB_HOST": "fakehost", "MONGO_DB_USER": "admin",
			"MONGO_DB_PASS": "admin123",
			"MONGO_DB_PORT": "27017"}}, expectedStr: "could not connect to mongoDB"},
	}

	for _, tc := range tcs {
		g := New()
		b := new(bytes.Buffer)

		g.Logger = log.NewMockLogger(b)
		initializeMongoDB(tc.configLoc, g)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tc.expectedStr, b.String())
		}
	}
}

func Test_initializeCassandra(t *testing.T) {
	// this is done to so that it doesnt affects the other tests related to cassandra
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	config.NewGoDotEnvProvider(logger, "../../configs")

	tcs := []struct {
		configLoc   Config
		expectedStr string
	}{
		{mockConfig{}, "Connected to cassandra"},
		{configLoc: &config.MockConfig{Data: map[string]string{"CASS_DB_HOST": "cassandra", "CASS_DB_PORT": "2003"}}},
	}

	for _, tc := range tcs {
		g := &Gofr{}
		g.Logger = logger

		initializeCassandra(mockConfig{}, g)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tc.expectedStr, b.String())
		}
	}
}

func Test_initializeCassandra_InvalidDialect(t *testing.T) {
	c := config.MockConfig{Data: map[string]string{"CASS_DB_DIALECT": "invalid", "CASS_DB_HOST": "localhost", "CASS_DB_PORT": "20112"}}

	expectedLog := "invalid dialect"
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	g := &Gofr{}
	g.Logger = logger

	initializeCassandra(&c, g)

	if !strings.Contains(b.String(), expectedLog) {
		t.Errorf("FAILED, expected: `%v` in the logs, got: %v", expectedLog, b.String())
	}
}

func Test_getYcqlConfigs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	cfg := &config.MockConfig{
		Data: map[string]string{
			"CASS_DB_DIALECT":  "YCQL",
			"CASS_DB_PASS":     c.Get("YCQL_DB_PASS"),
			"CASS_DB_USER":     c.Get("YCQL_DB_USER"),
			"CASS_DB_PORT":     c.Get("YCQL_DB_PORT"),
			"CASS_DB_KEYSPACE": c.Get("CASS_DB_KEYSPACE"),
			"CASS_DB_TIMEOUT":  c.Get("CASS_DB_TIMEOUT"),
		},
	}

	testCases := []struct {
		host        string
		expectedStr string
	}{
		{"localhost", "Connected to YCQL"},
		{"invalidhost", "could not connect to YCQL"},
	}

	for i, tc := range testCases {
		b.Reset()

		g := &Gofr{}
		g.Logger = logger

		cfg.Data["CASS_DB_HOST"] = tc.host

		initializeCassandra(cfg, g)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("FAILED case`%v`, expected: `%v` in the logs, got: %v", i, tc.expectedStr, b.String())
		}
	}
}

func Test_PubSub(t *testing.T) {
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	conf := config.NewGoDotEnvProvider(logger, "../../configs")

	g := &Gofr{Logger: logger}

	testCases := []struct {
		configLoc   Config
		expectedStr string
	}{
		{mockConfig{}, "Kafka initialized"},
		{&config.MockConfig{Data: map[string]string{
			"PUBSUB_BACKEND":           "google",
			"GOOGLE_TOPIC_NAME":        conf.Get("GOOGLE_TOPIC_NAME"),
			"GOOGLE_PROJECT_ID":        conf.Get("GOOGLE_PROJECT_ID"),
			"GOOGLE_SUBSCRIPTION_NAME": conf.Get("GOOGLE_SUBSCRIPTION_NAME"),
		}}, "Google PubSub initialized"},
	}

	for i, tc := range testCases {
		b.Reset()
		initializePubSub(tc.configLoc, logger, g)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("[FAILED %v], expected: `%v` in the logs, got: %v", i, tc.expectedStr, b.String())
		}
	}
}

func Test_PubSub_Eventhub(t *testing.T) {
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	conf := config.NewGoDotEnvProvider(logger, "../../configs")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{
			"subject": "gofr-value",
			"version": 3,
			"id":      303,
			"schema": `{"type":"record","name":"person","fields":[{"name":"Id","type":"string"},
						{"name":"Name","type":"string"},{"name":"Email","type":"string"}]}`,
		}

		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	g := &Gofr{Logger: logger}

	testCases := []struct {
		configLoc   Config
		expectedStr string
	}{
		{&config.MockConfig{Data: map[string]string{
			"EVENTHUB_NAMESPACE":  "zsmisc-dev",
			"EVENTHUB_NAME":       "healthcheck",
			"AZURE_CLIENT_ID":     conf.Get("AZURE_CLIENT_ID"),
			"AZURE_CLIENT_SECRET": conf.Get("AZURE_CLIENT_SECRET"),
			"AZURE_TENANT_ID":     conf.Get("AZURE_TENANT_ID"),
			"PUBSUB_BACKEND":      "EVENTHUB",
		}}, "Azure Eventhub initialized"},
		{&config.MockConfig{Data: map[string]string{
			"EVENTHUB_NAMESPACE":  "zsmisc-dev",
			"EVENTHUB_NAME":       "healthcheck",
			"AZURE_CLIENT_ID":     conf.Get("AZURE_CLIENT_ID"),
			"AZURE_CLIENT_SECRET": conf.Get("AZURE_CLIENT_SECRET"),
			"AZURE_TENANT_ID":     conf.Get("AZURE_TENANT_ID"),
			"PUBSUB_BACKEND":      "EVENTHUB",
			"AVRO_SCHEMA_URL":     ts.URL,
		}}, "Avro initialized"},
	}

	for i, tc := range testCases {
		b.Reset()
		initializePubSub(tc.configLoc, logger, g)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("[FAILED %v], expected: `%v` in the logs, got: %v", i, tc.expectedStr, b.String())
		}
	}
}

func Test_Notifier(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	conf := config.NewGoDotEnvProvider(logger, "../../configs")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{
			"subject": "gofr-value",
			"version": 3,
			"id":      303,
			"schema": `{"type":"record","name":"person","fields":[{"name":"Id","type":"string"},
						{"name":"Name","type":"string"},{"name":"Email","type":"string"}]}`,
		}

		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	g := &Gofr{Logger: logger}

	testCases := []struct {
		configLoc   Config
		expectedStr string
	}{
		{&config.MockConfig{Data: map[string]string{
			"EVENTHUB_NAMESPACE": "zsmisc-dev",
			"EVENTHUB_NAME":      "healthcheck",
			"AccessKeyID":        conf.Get("SNS_ACCESS_KEY"),
			"SecretAccessKey":    conf.Get("SNS_SECRET_ACCESS_KEY"),
			"Region":             conf.Get("SNS_REGION"),
			"NOTIFIER_BACKEND":   "SNS",
			"AVRO_SCHEMA_URL":    ts.URL,
		}}, "AWS SNS initialized"},
	}

	for i, tc := range testCases {
		b.Reset()
		initializeNotifiers(tc.configLoc, g)

		assert.Contains(t, b.String(), tc.expectedStr, "[FAILED %v], expected: `%v` in the logs, got: %v", i, tc.expectedStr, b.String())
	}
}

func Test_initializeAvro(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{
			"subject": "gofr-value",
			"version": 3,
			"id":      303,
			"schema": `{"type":"record","name":"person","fields":[{"name":"Id","type":"string"},
			{"name":"Name","type":"string"},{"name":"Email","type":"string"}]}`,
		}

		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	g := &Gofr{Logger: logger}
	topic := c.Get("KAFKA_TOPIC") // CSV string
	topics := strings.Split(topic, ",")
	kafkaCfg := &kafka.Config{
		Brokers: c.Get("KAFKA_HOSTS"),
		Topics:  topics,
	}
	kafkaObj, _ := kafka.New(kafkaCfg, logger)
	tests := []struct {
		c           Config
		ps          pubsub.PublisherSubscriber
		expectedStr string
	}{
		{&config.MockConfig{Data: map[string]string{"AVRO_SCHEMA_URL": ts.URL, "AVRO_SUBJECT": "gofr-value"}},
			kafkaObj, "Avro initialized!"},
		{&config.MockConfig{Data: map[string]string{"AVRO_SCHEMA_URL": ts.URL, "AVRO_SUBJECT": "gofr-value"}},
			nil, "Kafka/Eventhub not present, cannot use Avro"},
		{&config.MockConfig{Data: map[string]string{"AVRO_SCHEMA_URL": "", "AVRO_SUBJECT": "gofr-value"}},
			kafkaObj, "Schema registry URL is required for Avro"},
	}

	for _, tt := range tests {
		g.PubSub = tt.ps
		avroConfig := avroConfigFromEnv(tt.c, "")
		initializeAvro(avroConfig, g)

		if !strings.Contains(b.String(), tt.expectedStr) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tt.expectedStr, b.String())
		}
	}
}

func Test_initializeSolr(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	testCases := []struct {
		configLoc   config.MockConfig
		expectedStr string
	}{
		{
			config.MockConfig{Data: map[string]string{
				"SOLR_HOST": c.Get("SOLR_HOST"),
				"SOLR_PORT": c.Get("SOLR_PORT"),
			}},
			"Solr connected",
		},
		{
			config.MockConfig{Data: map[string]string{
				"SOLR_HOST": "",
				"SOLR_PORT": "",
			}},
			"",
		},
	}

	g := &Gofr{Logger: logger}

	for _, tc := range testCases {
		mockConfig := tc.configLoc

		initializeSolr(&mockConfig, g)

		if !strings.Contains(b.String(), tc.expectedStr) {
			t.Errorf("FAILED, expected: `%v` in the logs, got: %v", tc.expectedStr, b.String())
		}

		b = new(bytes.Buffer)
	}
}

func Test_GofrCMDConfig(t *testing.T) {
	g := NewCMD()
	if g.Redis == nil {
		t.Errorf("expected redis to be connected through configs")
	}
}

func Test_initializeEventBridge(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := &config.MockConfig{
		Data: map[string]string{
			"EVENT_BRIDGE_REGION": "us-east-1",
			"EVENT_BRIDGE_BUS":    "Gofr",
			"EVENT_BRIDGE_SOURCE": "Gofr-application",
		},
	}
	g := &Gofr{Logger: logger}
	initializeEventBridge(c, logger, g)

	assert.Contains(t, b.String(), "AWS EventBridge initialized successfully")
}

func Test_initializeAvroFromConfigs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"subject": "gofr-value",
			"version": 3,
			"id":      303,
			"schema": `{"type":"record","name":"person","fields":[{"name":"Id","type":"string"},
						{"name":"Name","type":"string"},{"name":"Email","type":"string"}]}`,
		}

		body, _ := json.Marshal(res)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(body)
	}))

	cfg := &avro.Config{
		URL:     ts.URL,
		Subject: "gofr-value",
	}

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	topic := c.Get("KAFKA_TOPIC")
	topics := strings.Split(topic, ",")
	kafkaCfg := &kafka.Config{
		Brokers: c.Get("KAFKA_HOSTS"),
		Topics:  topics,
	}
	kafkaObj, _ := kafka.New(kafkaCfg, logger)
	testcases := []struct {
		desc   string
		ps     pubsub.PublisherSubscriber
		expErr error
	}{
		{"Successful connection", kafkaObj, nil},
		{"Empty pubsub", nil, errors.DataStoreNotInitialized{DBName: "Avro", Reason: "Kafka/Eventhub not provided"}},
	}

	for i, tc := range testcases {
		_, err := initializeAvroFromConfigs(cfg, tc.ps)
		assert.Equal(t, tc.expErr, err, "Test[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_InitializeDynamoDBFromConfig(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	cfg := config.NewGoDotEnvProvider(logger, "../../configs")

	conn, err := InitializeDynamoDBFromConfig(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed.")
}

func Test_InitializeRedisFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	cfg := config.NewGoDotEnvProvider(logger, "../../configs")

	conn, err := InitializeRedisFromConfigs(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializeGORMFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	cfg := config.NewGoDotEnvProvider(logger, "../../configs")

	conn, err := InitializeGORMFromConfigs(cfg, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializeMongoDBFromConfigs(t *testing.T) {
	var cfg mockConfig

	logger := log.NewMockLogger(io.Discard)

	conn, err := InitializeMongoDBFromConfigs(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func TestInitializeSolrFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	cfg := &config.MockConfig{Data: map[string]string{
		"PRE_SOLR_HOST": c.Get("SOLR_HOST"),
		"PRE_SOLR_PORT": c.Get("SOLR_PORT"),
	}}

	conn, err := InitializeSolrFromConfigs(cfg, "PRE")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializeElasticSearchFromConfigs(t *testing.T) {
	cfg := &config.MockConfig{Data: map[string]string{"PRE_ELASTIC_SEARCH_HOST": "localhost",
		"PRE_ELASTIC_SEARCH_PORT": "2012"}}
	logger := log.NewMockLogger(io.Discard)

	conn, err := InitializeElasticSearchFromConfigs(cfg, logger, "PRE")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializeCassandraFromConfigs(t *testing.T) {
	var cfg mockConfig

	logger := log.NewMockLogger(io.Discard)

	conn, err := InitializeCassandraFromConfigs(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializeYCQLFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	cfg := &config.MockConfig{
		Data: map[string]string{
			"PRE_CASS_DB_DIALECT":  "YCQL",
			"PRE_CASS_DB_PASS":     c.Get("YCQL_DB_PASS"),
			"PRE_CASS_DB_USER":     c.Get("YCQL_DB_USER"),
			"PRE_CASS_DB_PORT":     c.Get("YCQL_DB_PORT"),
			"PRE_CASS_DB_KEYSPACE": c.Get("CASS_DB_KEYSPACE"),
			"PRE_CASS_DB_TIMEOUT":  c.Get("CASS_DB_TIMEOUT"),
			"PRE_CASS_DB_HOST":     "localhost",
		},
	}

	conn, err := InitializeYCQLFromConfigs(cfg, logger, "PRE")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_initializeEventhubFromConfigs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	cfg := &config.MockConfig{Data: map[string]string{
		"PRE_EVENTHUB_NAMESPACE":  "zsmisc-dev",
		"PRE_EVENTHUB_NAME":       "healthcheck",
		"PRE_AZURE_CLIENT_ID":     c.Get("AZURE_CLIENT_ID"),
		"PRE_AZURE_CLIENT_SECRET": c.Get("AZURE_CLIENT_SECRET"),
		"PRE_AZURE_TENANT_ID":     c.Get("AZURE_TENANT_ID"),
		"PRE_PUBSUB_BACKEND":      "EVENTHUB",
	}}

	conn, err := initializeEventhubFromConfigs(cfg, "PRE")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_initializeEventBridgeFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	cfg := &config.MockConfig{
		Data: map[string]string{
			"EVENT_BRIDGE_REGION": "us-east-1",
			"EVENT_BRIDGE_BUS":    "Gofr",
			"EVENT_BRIDGE_SOURCE": "Gofr-application",
		},
	}

	conn, err := initializeEventBridgeFromConfigs(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializeKafkaFromConfigs(t *testing.T) {
	var cfg mockConfig

	logger := log.NewMockLogger(io.Discard)

	conn, err := initializeKafkaFromConfigs(cfg, logger, "")
	if err != nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", nil, err)
	}

	assert.NotNil(t, conn, "Test case failed")
}

func Test_InitializePubSubFromConfigs(t *testing.T) {
	cfg := &config.MockConfig{Data: map[string]string{"PRE_PUBSUB_BACKEND": ""}}
	expErr := errors.DataStoreNotInitialized{DBName: "PubSub", Reason: "pubsub backend not provided"}

	ps, err := InitializePubSubFromConfigs(cfg, log.NewMockLogger(io.Discard), "PRE")

	assert.Equal(t, nil, ps, "Test case failed")
	assert.Equal(t, expErr, err, "Test case failed")
}

func Test_InitializeAWSSNSFromConfigs(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	conf := config.NewGoDotEnvProvider(logger, "../../configs")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{
			"subject": "gofr-value",
			"version": 3,
			"id":      303,
			"schema": `{"type":"record","name":"person","fields":[{"name":"Id","type":"string"},
						{"name":"Name","type":"string"},{"name":"Email","type":"string"}]}`,
		}

		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))
	cfg := &config.MockConfig{Data: map[string]string{
		"PRE_EVENTHUB_NAMESPACE": "zsmisc-dev",
		"PRE_EVENTHUB_NAME":      "healthcheck",
		"PRE_AccessKeyID":        conf.Get("SNS_ACCESS_KEY"),
		"PRE_SecretAccessKey":    conf.Get("SNS_SECRET_ACCESS_KEY"),
		"PRE_Region":             conf.Get("SNS_REGION"),
		"PRE_NOTIFIER_BACKEND":   "SNS",
		"PRE_AVRO_SCHEMA_URL":    ts.URL,
	}}

	conn, err := InitializeAWSSNSFromConfigs(cfg, "PRE")

	assert.NotNil(t, conn, "Test case failed")
	assert.Equal(t, nil, err, "Test case failed")
}

func Test_InitializeSQLFromConfigs(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	cfg := config.NewGoDotEnvProvider(logger, "../../configs")
	conn, err := InitializeSQLFromConfigs(cfg, "")

	assert.NotNil(t, conn, "Test case failed")
	assert.Equal(t, nil, err, "Test case failed")
}

func Test_RemoteConfig(t *testing.T) {
	cfg := &config.MockConfig{Data: map[string]string{"REMOTE_CONFIG_URL": "http://dummy", "DB_NAME": "mock-db"}}
	app := NewWithConfig(cfg)
	assert.IsType(t, &config.RemoteConfig{}, app.Config, "Test case failed.")
}

func Test_initializeGooglePubSub(t *testing.T) {
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	cfg := config.NewGoDotEnvProvider(logger, "../../configs")
	app := New()
	app.Logger = logger

	testCases := []struct {
		desc    string
		configs Config
		expLog  string
	}{
		{"Success Case: correct credentials are given", &config.MockConfig{Data: map[string]string{
			"PUBSUB_BACKEND":           "google",
			"GOOGLE_TOPIC_NAME":        cfg.Get("GOOGLE_TOPIC_NAME"),
			"GOOGLE_PROJECT_ID":        cfg.Get("GOOGLE_PROJECT_ID"),
			"GOOGLE_SUBSCRIPTION_NAME": cfg.Get("GOOGLE_SUBSCRIPTION_NAME"),
			"GOOGLE_TIMEOUT_DURATION":  "5",
		}}, "Google PubSub initialized"},
		{"Failure Case: incorrect credentials are given", &config.MockConfig{Data: map[string]string{
			"PUBSUB_BACKEND":           "google",
			"GOOGLE_TOPIC_NAME":        cfg.Get("GOOGLE_TOPIC_NAME"),
			"GOOGLE_PROJECT_ID":        "",
			"GOOGLE_SUBSCRIPTION_NAME": cfg.Get("GOOGLE_SUBSCRIPTION_NAME"),
			"GOOGLE_TIMEOUT_DURATION":  "5",
		}}, "Cannot connect to google pubsub:"},
	}

	for i, tc := range testCases {
		initializeGooglePubSub(tc.configs, app)

		assert.Containsf(t, b.String(), tc.expLog, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func Test_initializeClickHouse(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	c := config.NewGoDotEnvProvider(logger, "../../configs")

	hostName := c.Get("CLICKHOUSE_HOST")
	port := c.Get("CLICKHOUSE_PORT")

	testcases := []struct {
		host        string
		port        string
		expectedLog string
	}{
		{"", "", ""},
		{"incorrect-url", "7", "could not connect to ClickHouse"},
		{hostName, port, "ClickHouse connected"},
	}

	for i, tc := range testcases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		mockConfig := config.MockConfig{
			Data: map[string]string{"CLICKHOUSE_HOST": tc.host, "CLICKHOUSE_USER": c.Get("CLICKHOUSE_USER"),
				"CLICKHOUSE_PASSWORD": c.Get("CLICKHOUSE_PASSWORD"), "CLICKHOUSE_DB": c.Get("CLICKHOUSE_NAME"),
				"CLICKHOUSE_PORT": tc.port, "CLICKHOUSE_MAX_OPEN_CONN": c.Get("CLICKHOUSE_MAX_OPEN_CONN"),
				"CLICKHOUSE_MAX_IDLE_CONN": c.Get("CLICKHOUSE_MAX_IDLE_CONN"), "CLICKHOUSE_MAX_CONN_LIFETIME": c.Get("CLICKHOUSE_MAX_CONN_LIFETIME"),
			},
		}

		g := NewWithConfig(&mockConfig)
		g.Logger = logger

		initializeClickHouseDB(&mockConfig, g)

		if !strings.Contains(b.String(), tc.expectedLog) {
			t.Errorf("[TESTCASE %d] Failed. Got: %v\tExpected: %v\n", i+1, b.String(), tc.expectedLog)
		}
	}
}
