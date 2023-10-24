package gofr

import (
	"bytes"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub/google"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/log"
)

func Test_kafkaRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	kafkaConfig := kafkaConfigFromEnv(c, "")
	avroConfig := avroConfigFromEnv(c, "")

	kafkaConfig.ConnRetryDuration = 1
	k.Logger = logger

	kafkaRetry(kafkaConfig, avroConfig, &k)

	if !k.PubSub.IsSet() {
		t.Errorf("FAILED, expected: Kafka initialized successfully, got: kafka initialization failed")
	}
}

func Test_eventhubRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	conf := config.NewGoDotEnvProvider(logger, "../../configs")
	c := &config.MockConfig{Data: map[string]string{
		"EVENTHUB_NAME":       "healthcheck",
		"EVENTHUB_NAMESPACE":  "",
		"AZURE_CLIENT_ID":     conf.Get("AZURE_CLIENT_ID"),
		"AZURE_CLIENT_SECRET": conf.Get("AZURE_CLIENT_SECRET"),
		"AZURE_TENANT_ID":     conf.Get("AZURE_TENANT_ID"),
		"PUBSUB_BACKEND":      "EVENTHUB",
	}}
	eventhubConfig := eventhubConfigFromEnv(c, "")

	eventhubConfig.ConnRetryDuration = 1
	k.Logger = logger

	eventhubRetry(&eventhubConfig, nil, &k)

	if !k.PubSub.IsSet() {
		t.Errorf("FAILED, expected: Eventhub initialized successfully, got: Eventhub initialization failed")
	}
}

func Test_mongoRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	enableSSl, _ := strconv.ParseBool(c.Get("MONGO_DB_ENABLE_SSL"))
	retryWrites, _ := strconv.ParseBool(c.Get("MONGO_DB_RETRY_WRITES"))
	mongoCfg := datastore.MongoConfig{
		HostName:          c.Get("MONGO_DB_HOST"),
		Port:              c.Get("MONGO_DB_PORT"),
		Username:          c.Get("MONGO_DB_USER"),
		Password:          c.Get("MONGO_DB_PASS"),
		Database:          c.Get("MONGO_DB_NAME"),
		SSL:               enableSSl,
		RetryWrites:       retryWrites,
		ConnRetryDuration: 1,
	}

	k.Logger = logger

	mongoRetry(&mongoCfg, &k)

	if !k.MongoDB.IsSet() {
		t.Errorf("FAILED, expected: MongoDB initialized successfully, got: MongoDB initialization failed")
	}
}

func Test_cassandraRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	cassandraCfg := cassandraConfigFromEnv(c, "")

	cassandraCfg.ConnRetryDuration = 1
	k.Logger = logger

	cassandraRetry(cassandraCfg, &k)

	if k.Cassandra.Session == nil {
		t.Errorf("FAILED, expected: Cassandra initialized successfully, got: cassandra initialization failed")
	}
}

func Test_ycqlRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	cassandraCfg := getYcqlConfigs(c, "")

	cassandraCfg.Port, _ = strconv.Atoi(c.Get("YCQL_DB_PORT"))
	cassandraCfg.Password = c.Get("YCQL_DB_PASS")
	cassandraCfg.Username = c.Get("YCQL_DB_USER")
	cassandraCfg.ConnRetryDuration = 1
	cassandraCfg.Hosts = c.Get("CASS_DB_HOST")
	k.Logger = logger

	yclRetry(&cassandraCfg, &k)

	if k.YCQL.Session == nil {
		t.Errorf("FAILED, expected: Ycql initialized successfully, got: Ycql initialization failed")
	}
}

func Test_ormRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	dc := datastore.DBConfig{
		HostName:          c.Get("DB_HOST"),
		Username:          c.Get("DB_USER"),
		Password:          c.Get("DB_PASSWORD"),
		Database:          c.Get("DB_NAME"),
		Port:              c.Get("DB_PORT"),
		Dialect:           c.Get("DB_DIALECT"),
		SSL:               c.Get("DB_SSL"),
		ORM:               c.Get("DB_ORM"),
		ConnRetryDuration: 1,
	}

	k.Logger = logger

	ormRetry(&dc, &k)

	sqlDB, err := k.GORM().DB()

	assert.NoError(t, err, "FAILED, expected: successful initialisation of ORM, got: orm initialisation failed")

	err = sqlDB.Ping()

	assert.NoError(t, err, "FAILED, expected: successful initialisation of ORM, got: orm initialisation failed")
}

// Testing sqlx retry mechanism
func Test_sqlxRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	dc := datastore.DBConfig{
		HostName:          c.Get("DB_HOST"),
		Username:          c.Get("DB_USER"),
		Password:          c.Get("DB_PASSWORD"),
		Database:          c.Get("DB_NAME"),
		Port:              c.Get("DB_PORT"),
		Dialect:           c.Get("DB_DIALECT"),
		SSL:               c.Get("DB_SSL"),
		ORM:               c.Get("DB_ORM"),
		ConnRetryDuration: 1,
	}

	k.Logger = logger

	sqlxRetry(&dc, &k)

	if k.SQLX() == nil || (k.SQLX() != nil && k.SQLX().Ping() != nil) {
		t.Errorf("FAILED, expected: SQLX initialized successfully, got: sqlx initialization failed")
	}
}

func Test_redisRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	redisConfig := datastore.RedisConfig{
		HostName:                c.Get("REDIS_HOST"),
		Port:                    c.Get("REDIS_PORT"),
		ConnectionRetryDuration: 1,
	}

	redisConfig.Options = new(redis.Options)
	redisConfig.Options.Addr = redisConfig.HostName + ":" + redisConfig.Port
	k.Logger = logger

	redisRetry(&redisConfig, &k)

	if !k.Redis.IsSet() {
		t.Errorf("FAILED, expected: Redis initialized successfully, got: redis initialization failed")
	}
}

func Test_elasticSearchRetry(t *testing.T) {
	k := Gofr{Logger: log.NewMockLogger(io.Discard)}

	cfg := &datastore.ElasticSearchCfg{Ports: []int{2012}, ConnectionRetryDuration: 1, Host: "localhost"}

	elasticSearchRetry(cfg, &k)

	if k.Elasticsearch.Client == nil {
		t.Errorf("Expected: successful initialization, Got: initialization failed")
	}
}

func Test_AWSSNSRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	awsSNSConfig := awsSNSConfigFromEnv(c, "")

	awsSNSConfig.ConnRetryDuration = 1
	k.Logger = logger

	awsSNSRetry(&awsSNSConfig, &k)

	assert.True(t, k.Notifier.IsSet(), "FAILED, expected: AwsSNS initialized successfully, got: AwsSNS initialization failed")
}

func Test_dynamoRetry(t *testing.T) {
	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	dynamoConfig := datastore.DynamoDBConfig{
		Region:            c.Get("DYNAMODB_REGION"),
		Endpoint:          c.Get("DYNAMODB_ENDPOINT_URL"),
		SecretAccessKey:   c.Get("DYNAMODB_SECRET_ACCESS_KEY"),
		AccessKeyID:       c.Get("DYNAMODB_ACCESS_KEY_ID"),
		ConnRetryDuration: 1,
	}

	k.Logger = logger

	dynamoRetry(dynamoConfig, &k)

	if k.DynamoDB.DynamoDB == nil {
		t.Errorf("FAILED, expected: DynamoDB initialized successfully, got: DynamoDB initialization failed")
	}
}

func Test_AWSEventBridgeRetry(t *testing.T) {
	var k Gofr

	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	k.Logger = logger
	c := config.NewGoDotEnvProvider(logger, "../../configs")
	cfg := eventbridgeConfigFromEnv(c, logger, "")
	cfg.ConnRetryDuration = 1

	go eventbridgeRetry(cfg, &k)

	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)

		if k.PubSub != nil {
			break
		}
	}

	assert.Contains(t, b.String(), "AWS EventBridge initialized successfully")
}

func Test_googlePubsubRetry(t *testing.T) {
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	var k Gofr

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../configs")

	g := google.Config{
		TopicName: c.Get("GOOGLE_TOPIC_NAME"),
		ProjectID: c.Get("GOOGLE_PROJECT_ID"),
		SubscriptionDetails: &google.Subscription{
			Name: c.Get("GOOGLE_SUBSCRIPTION_NAME"),
		},
		TimeoutDuration:   30,
		ConnRetryDuration: 1,
	}

	k.Logger = logger

	go googlePubsubRetry(g, &k)

	time.Sleep(30 * time.Second)

	if !k.PubSub.IsSet() {
		t.Errorf("FAILED, expected: GooglePubsub initialized successfully, got: g Pubsub initialization failed")
	}
}
