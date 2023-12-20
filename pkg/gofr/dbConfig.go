package gofr

import (
	"crypto/tls"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gocql/gocql"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/datastore/pubsub/eventbridge"
	"gofr.dev/pkg/datastore/pubsub/eventhub"
	"gofr.dev/pkg/datastore/pubsub/google"
	"gofr.dev/pkg/datastore/pubsub/kafka"
	"gofr.dev/pkg/log"
	awssns "gofr.dev/pkg/notifier/aws-sns"
)

// redisConfigFromEnv reads configs and returns all the redis configs
func redisConfigFromEnv(c Config, prefix string) datastore.RedisConfig {
	if prefix != "" {
		prefix += "_"
	}

	ssl := false
	if strings.EqualFold(c.Get(prefix+"REDIS_SSL"), "true") {
		ssl = true
	}

	redisConfig := datastore.RedisConfig{
		HostName:                c.Get(prefix + "REDIS_HOST"),
		Password:                c.Get(prefix + "REDIS_PASSWORD"),
		Port:                    c.Get(prefix + "REDIS_PORT"),
		DB:                      getRedisDB(c.Get(prefix + "REDIS_DB")),
		ConnectionRetryDuration: getRetryDuration(c.Get(prefix + "REDIS_CONN_RETRY")),
		SSL:                     ssl,
	}

	// set redis connection pooling configs
	redisConfig.Options = &redis.Options{}
	redisConfig.Options.PoolSize, _ = strconv.Atoi(c.Get(prefix + "REDIS_POOL_SIZE"))
	redisConfig.Options.MinIdleConns, _ = strconv.Atoi(c.Get("REDIS_MIN_IDLE_CONN"))

	maxConnAge, _ := strconv.Atoi(c.Get(prefix + "REDIS_MAX_CONN_AGE"))
	redisConfig.Options.MaxConnAge = time.Duration(maxConnAge) * time.Second

	poolTimeout, _ := strconv.Atoi(c.Get(prefix + "REDIS_POOL_TIMEOUT"))
	redisConfig.Options.PoolTimeout = time.Duration(poolTimeout) * time.Second

	idleTimeout, _ := strconv.Atoi(c.Get(prefix + "REDIS_IDLE_TIMEOUT"))
	redisConfig.Options.IdleTimeout = time.Duration(idleTimeout) * time.Second

	return redisConfig
}

// cassandraDBConfigFromEnv returns configuration from environment variables to client so it can connect to cassandra
func cassandraConfigFromEnv(c Config, prefix string) *datastore.CassandraCfg {
	if prefix != "" {
		prefix += "_"
	}

	cassandraTimeout, err := strconv.Atoi(c.Get(prefix + "CASS_DB_TIMEOUT"))
	if err != nil {
		// setting default timeout of 600 milliseconds
		cassandraTimeout = 600
	}

	cassandraConnTimeout, err := strconv.Atoi(c.Get(prefix + "CASS_DB_CONN_TIMEOUT"))
	if err != nil {
		// setting default timeout of 600 milliseconds
		cassandraConnTimeout = 600
	}

	cassandraPort, err := strconv.Atoi(c.Get(prefix + "CASS_DB_PORT"))
	if err != nil {
		// if any error, setting default port
		cassandraPort = 9042
	}

	const retries = 5

	cassandraConfig := datastore.CassandraCfg{
		Hosts:               c.Get(prefix + "CASS_DB_HOST"),
		Port:                cassandraPort,
		Username:            c.Get(prefix + "CASS_DB_USER"),
		Password:            c.Get(prefix + "CASS_DB_PASS"),
		Keyspace:            c.Get(prefix + "CASS_DB_KEYSPACE"),
		Consistency:         c.Get(prefix + "CASS_DB_CONSISTENCY"),
		Timeout:             cassandraTimeout,
		ConnectTimeout:      cassandraConnTimeout,
		RetryPolicy:         &gocql.SimpleRetryPolicy{NumRetries: retries},
		TLSVersion:          setTLSVersion(c.Get(prefix + "CASS_DB_TLS_VERSION")),
		HostVerification:    getBool(c.Get(prefix + "CASS_DB_HOST_VERIFICATION")),
		ConnRetryDuration:   getRetryDuration(c.Get(prefix + "CASS_CONN_RETRY")),
		CertificateFile:     c.Get(prefix + "CASS_DB_CERTIFICATE_FILE"),
		KeyFile:             c.Get(prefix + "CASS_DB_KEY_FILE"),
		RootCertificateFile: c.Get(prefix + "CASS_DB_ROOT_CERTIFICATE_FILE"),
		InsecureSkipVerify:  getBool(c.Get(prefix + "CASS_DB_INSECURE_SKIP_VERIFY")),
		DataCenter:          c.Get(prefix + "DATA_CENTER"),
	}

	return &cassandraConfig
}

func getBool(val string) bool {
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}

	return boolVal
}

func setTLSVersion(version string) uint16 {
	if version == "10" {
		return tls.VersionTLS10
	} else if version == "11" {
		return tls.VersionTLS11
	} else if version == "13" {
		return tls.VersionTLS13
	}

	return tls.VersionTLS12
}

func sqlDBConfigFromEnv(c Config, prefix string) *datastore.DBConfig {
	if prefix != "" {
		prefix += "_"
	}

	openC, _ := strconv.Atoi(c.Get(prefix + "DB_MAX_OPEN_CONN"))
	idleC, _ := strconv.Atoi(c.Get(prefix + "DB_MAX_IDLE_CONN"))
	connL, _ := strconv.Atoi(c.Get(prefix + "DB_MAX_CONN_LIFETIME"))

	return &datastore.DBConfig{
		HostName:          c.Get(prefix + "DB_HOST"),
		Username:          c.Get(prefix + "DB_USER"),
		Password:          c.Get(prefix + "DB_PASSWORD"),
		Database:          c.Get(prefix + "DB_NAME"),
		Port:              c.Get(prefix + "DB_PORT"),
		Dialect:           c.Get(prefix + "DB_DIALECT"),
		SSL:               c.Get(prefix + "DB_SSL"),
		ORM:               c.Get(prefix + "DB_ORM"),
		CertificateFile:   c.Get(prefix + "DB_CERTIFICATE_FILE"),
		KeyFile:           c.Get(prefix + "DB_KEY_FILE"),
		CACertificateFile: c.Get(prefix + "DB_CA_CERTIFICATE_FILE"),
		ConnRetryDuration: getRetryDuration(c.Get(prefix + "DB_CONN_RETRY")),
		MaxOpenConn:       openC,
		MaxIdleConn:       idleC,
		MaxConnLife:       connL,
	}
}

// mongoDBConfigFromEnv returns configuration from environment variables to client so it can connect to MongoDB
func mongoDBConfigFromEnv(c Config, prefix string) *datastore.MongoConfig {
	if prefix != "" {
		prefix += "_"
	}

	enableSSl, _ := strconv.ParseBool(c.Get(prefix + "MONGO_DB_ENABLE_SSL"))
	retryWrites, _ := strconv.ParseBool(c.Get(prefix + "MONGO_DB_RETRY_WRITES"))

	mongoConfig := datastore.MongoConfig{
		HostName:          c.Get(prefix + "MONGO_DB_HOST"),
		Port:              c.Get(prefix + "MONGO_DB_PORT"),
		Username:          c.Get(prefix + "MONGO_DB_USER"),
		Password:          c.Get(prefix + "MONGO_DB_PASS"),
		Database:          c.Get(prefix + "MONGO_DB_NAME"),
		SSL:               enableSSl,
		RetryWrites:       retryWrites,
		ConnRetryDuration: getRetryDuration(c.Get(prefix + "MONGO_CONN_RETRY")),
	}

	return &mongoConfig
}

// kafkaDBConfigFromEnv returns configuration from environment variables to client so it can connect to kafka
func kafkaConfigFromEnv(c Config, prefix string) *kafka.Config {
	hosts := c.Get(prefix + "KAFKA_HOSTS") // CSV string
	topic := c.Get(prefix + "KAFKA_TOPIC") // CSV string
	retryFrequency, _ := strconv.Atoi(c.Get(prefix + "KAFKA_RETRY_FREQUENCY"))
	maxRetry, _ := strconv.Atoi(c.GetOrDefault(prefix+"KAFKA_MAX_RETRY", "10"))
	// consumer groupID generation using APP_NAME and APP_VERSION
	groupName := c.Get(prefix + "KAFKA_CONSUMERGROUP_NAME")
	if groupName == "" {
		groupName = c.GetOrDefault("APP_NAME", pkg.DefaultAppName) + "-" + c.GetOrDefault("APP_VERSION", pkg.DefaultAppVersion) + "-consumer"
	}

	disableautocommit, _ := strconv.ParseBool(c.GetOrDefault(prefix+"KAFKA_AUTOCOMMIT_DISABLE", "false"))

	// converting the CSV string to slice of string
	topics := strings.Split(topic, ",")
	config := &kafka.Config{
		Brokers: hosts,
		SASL: kafka.SASLConfig{
			User:      c.Get(prefix + "KAFKA_SASL_USER"),
			Password:  c.Get(prefix + "KAFKA_SASL_PASS"),
			Mechanism: c.Get(prefix + "KAFKA_SASL_MECHANISM"),
		},
		Topics:            topics,
		MaxRetry:          maxRetry,
		RetryFrequency:    retryFrequency,
		ConnRetryDuration: getRetryDuration(c.Get(prefix + "KAFKA_CONN_RETRY")),
		InitialOffsets:    kafka.OffsetOldest,
		GroupID:           groupName,
		DisableAutoCommit: disableautocommit,
	}

	offset := c.GetOrDefault(prefix+"KAFKA_CONSUMER_OFFSET", "OLDEST")

	switch offset {
	case "NEWEST":
		config.InitialOffsets = kafka.OffsetNewest
	default:
		config.InitialOffsets = kafka.OffsetOldest
	}

	return config
}

// elasticSearchConfigFromEnv returns configuration from environment variables to client so it can connect to elasticsearch
func elasticSearchConfigFromEnv(c Config, prefix string) datastore.ElasticSearchCfg {
	if prefix != "" {
		prefix += "_"
	}

	ports := make([]int, 0)
	portList := strings.Split(c.Get(prefix+"ELASTIC_SEARCH_PORT"), ",")

	for _, port := range portList {
		p, err := strconv.Atoi(strings.TrimSpace(port))
		if err != nil {
			continue
		}

		ports = append(ports, p)
	}

	return datastore.ElasticSearchCfg{
		Host:                    c.Get(prefix + "ELASTIC_SEARCH_HOST"),
		Ports:                   ports,
		Username:                c.Get(prefix + "ELASTIC_SEARCH_USER"),
		Password:                c.Get(prefix + "ELASTIC_SEARCH_PASS"),
		CloudID:                 c.Get(prefix + "ELASTIC_CLOUD_ID"),
		ConnectionRetryDuration: getRetryDuration(c.Get(prefix + "ELASTIC_SEARCH_CONN_RETRY")),
	}
}

func avroConfigFromEnv(c Config, prefix string) *avro.Config {
	return &avro.Config{
		URL:            c.Get(prefix + "AVRO_SCHEMA_URL"),
		Version:        c.Get(prefix + "AVRO_SCHEMA_VERSION"),
		Subject:        c.Get(prefix + "AVRO_SUBJECT"),
		SchemaUser:     c.Get(prefix + "AVRO_USER"),
		SchemaPassword: c.Get(prefix + "AVRO_PASSWORD"),
	}
}

func eventhubConfigFromEnv(c Config, prefix string) eventhub.Config {
	brokers := c.Get(prefix + "EVENTHUB_NAMESPACE")
	topics := strings.Split(c.Get(prefix+"EVENTHUB_NAME"), ",")

	return eventhub.Config{
		Namespace:         brokers,
		EventhubName:      topics[0],
		ClientID:          c.Get(prefix + "AZURE_CLIENT_ID"),
		ClientSecret:      c.Get(prefix + "AZURE_CLIENT_SECRET"),
		TenantID:          c.Get(prefix + "AZURE_TENANT_ID"),
		SharedAccessName:  c.Get(prefix + "EVENTHUB_SAS_NAME"),
		SharedAccessKey:   c.Get(prefix + "EVENTHUB_SAS_KEY"),
		ConnRetryDuration: getRetryDuration(c.Get(prefix + "EVENTHUB_CONN_RETRY")),
	}
}

func eventbridgeConfigFromEnv(c Config, logger log.Logger, prefix string) *eventbridge.Config {
	retryFrequency, _ := strconv.Atoi(c.Get(prefix + "EVENT_BRIDGE_RETRY_FREQUENCY"))
	akID := c.Get(prefix + "EVENT_BRIDGE_ACCESS_KEY_ID")
	secretAk := c.Get(prefix + "EVENT_BRIDGE_SECRET_ACCESS_KEY")

	if akID == "" {
		akID = c.Get(prefix + "EVENTBRIDGE_ACCESS_KEY_ID")
		secretAk = c.Get(prefix + "EVENTBRIDGE_SECRET_ACCESS_KEY")

		if akID != "" {
			logger.Warn("Usage of EVENTBRIDGE_ACCESS_KEY_ID and EVENTBRIDGE_SECRET_ACCESS_KEY is deprecated. " +
				"Instead use EVENT_BRIDGE_ACCESS_KEY_ID and EVENT_BRIDGE_SECRET_ACCESS_KEY")
		}
	}

	return &eventbridge.Config{
		ConnRetryDuration: retryFrequency,
		EventBus:          c.Get(prefix + "EVENT_BRIDGE_BUS"),
		EventSource:       c.Get(prefix + "EVENT_BRIDGE_SOURCE"),
		Region:            c.Get(prefix + "EVENT_BRIDGE_REGION"),
		AccessKeyID:       akID,
		SecretAccessKey:   secretAk,
	}
}

func awsSNSConfigFromEnv(c Config, prefix string) awssns.Config {
	if prefix != "" {
		prefix += "_"
	}

	return awssns.Config{
		AccessKeyID:     c.Get(prefix + "SNS_ACCESS_KEY"),
		SecretAccessKey: c.Get(prefix + "SNS_SECRET_ACCESS_KEY"),
		Region:          c.Get(prefix + "SNS_REGION"),
		TopicArn:        c.Get(prefix + "SNS_TOPIC_ARN"),
		Protocol:        strings.ToLower(c.Get(prefix + "SNS_PROTOCOL")),
		Endpoint:        c.Get(prefix + "SNS_ENDPOINT"),
	}
}

func googlePubSubConfigFromEnv(c Config, prefix string) google.Config {
	if prefix != "" {
		prefix += "_"
	}

	topicName := c.Get(prefix + "GOOGLE_TOPIC_NAME")
	projectID := c.Get(prefix + "GOOGLE_PROJECT_ID")
	connRetryDuration := getRetryDuration(c.Get(prefix + "GOOGLE_CONN_RETRY"))
	timeoutDuration := getTimoutDuration(c.Get(prefix + "GOOGLE_TIMEOUT_DURATION"))

	return google.Config{
		ProjectID:         projectID,
		TopicName:         topicName,
		ConnRetryDuration: connRetryDuration,
		TimeoutDuration:   timeoutDuration,
		SubscriptionDetails: &google.Subscription{
			Name: c.Get(prefix + "GOOGLE_SUBSCRIPTION_NAME"),
		},
	}
}

func dynamoDBConfigFromEnv(c Config, prefix string) datastore.DynamoDBConfig {
	if prefix != "" {
		prefix += "_"
	}

	return datastore.DynamoDBConfig{
		Region:            c.Get(prefix + "DYNAMODB_REGION"),
		Endpoint:          c.Get(prefix + "DYNAMODB_ENDPOINT_URL"),
		AccessKeyID:       c.Get(prefix + "DYNAMODB_ACCESS_KEY_ID"),
		SecretAccessKey:   c.Get(prefix + "DYNAMODB_SECRET_ACCESS_KEY"),
		ConnRetryDuration: getRetryDuration(c.Get(prefix + "DYNAMODB_CONN_RETRY")),
	}
}

func getTimoutDuration(envDuration string) int {
	timeoutDuration, _ := strconv.Atoi(envDuration)
	if timeoutDuration == 0 {
		// default duration 30 seconds
		timeoutDuration = 30
	}

	return timeoutDuration
}

func clickhouseDBConfigFromEnv(c Config, prefix string) *datastore.ClickHouseConfig {
	if prefix != "" {
		prefix += "_"
	}

	openC, _ := strconv.Atoi(c.Get(prefix + "CLICKHOUSE_MAX_OPEN_CONN"))
	idleC, _ := strconv.Atoi(c.Get(prefix + "CLICKHOUSE_MAX_IDLE_CONN"))
	connL, _ := strconv.Atoi(c.Get(prefix + "CLICKHOUSE_MAX_CONN_LIFETIME"))

	return &datastore.ClickHouseConfig{
		Host:              c.Get(prefix + "CLICKHOUSE_HOST"),
		Username:          c.Get(prefix + "CLICKHOUSE_USER"),
		Password:          c.Get(prefix + "CLICKHOUSE_PASSWORD"),
		Port:              c.Get(prefix + "CLICKHOUSE_PORT"),
		Database:          c.Get(prefix + "CLICKHOUSE_DB"),
		ConnRetryDuration: getRetryDuration(c.Get(prefix + "CLICKHOUSE_CONN_RETRY")),
		MaxOpenConn:       openC,
		MaxIdleConn:       idleC,
		MaxConnLife:       connL,
	}
}
