package gofr

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/datastore/pubsub/eventbridge"
	"gofr.dev/pkg/datastore/pubsub/eventhub"
	"gofr.dev/pkg/datastore/pubsub/google"
	"gofr.dev/pkg/datastore/pubsub/kafka"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/responder"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
	"gofr.dev/pkg/notifier"
	awssns "gofr.dev/pkg/notifier/aws-sns"
)

//nolint:gochecknoglobals // need to declare global variable to push metrics
var (
	frameworkInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zs_info",
		Help: "Gauge to count the pods running for each service and framework version",
	}, []string{"app", "framework"})

	_ = prometheus.Register(frameworkInfo)
)

// New creates a new instance of the Gofr object
func New() (k *Gofr) {
	logger := log.NewLogger()

	return NewWithConfig(config.NewGoDotEnvProvider(logger, getConfigFolder()))
}

// NewWithConfig creates a new instance of gofr object based on the configurations provided
//
//nolint:gocognit  // It's a sequence of initialization. Easier to understand this way.
func NewWithConfig(c Config) (k *Gofr) {
	// Here we do things based on what is provided by Config
	logger := log.NewLogger()

	gofr := &Gofr{
		Logger:         logger,
		Config:         c,
		DatabaseHealth: []HealthCheck{},
	}

	gofr.DataStore.Logger = logger

	appVers := c.Get("APP_VERSION")
	if appVers == "" {
		appVers = pkg.DefaultAppVersion

		logger.Warnf("APP_VERSION is not set. '%v' will be used in logs", pkg.DefaultAppVersion)
	}

	appName := c.Get("APP_NAME")
	if appName == "" {
		appName = pkg.DefaultAppName

		logger.Warnf("APP_NAME is not set.'%v' will be used in logs", pkg.DefaultAppName)
	}

	remoteConfigURL := c.Get("REMOTE_CONFIG_URL")
	if remoteConfigURL != "" {
		c = config.NewRemoteConfigProvider(c, remoteConfigURL, appName, logger)
		gofr.Config = c
	}

	frameworkInfo.WithLabelValues(appName+"-"+appVers, "gofr-"+log.GofrVersion).Set(1)

	s := NewServer(c, gofr)
	gofr.Server = s

	// HEADER VALIDATION
	enableHeaderValidation := c.Get("VALIDATE_HEADERS")
	enableHeaderValidation = strings.ToLower(enableHeaderValidation)

	if enableHeaderValidation == "true" {
		s.ValidateHeaders = true
	}

	// HTTP PORT
	p, err := strconv.Atoi(c.Get("HTTP_PORT"))
	s.HTTP.Port = p

	if err != nil || p <= 0 {
		s.HTTP.Port = 8000
	}

	// HTTPS Initialisation
	s.HTTPS.KeyFile = c.Get("KEY_FILE")
	s.HTTPS.CertificateFile = c.Get("CERTIFICATE_FILE")

	p, err = strconv.Atoi(c.Get("HTTPS_PORT"))
	s.HTTPS.Port = p

	if err != nil || p <= 0 {
		s.HTTPS.Port = 443
	}

	// set GRPC port from config
	p, err = strconv.Atoi(c.Get("GRPC_PORT"))
	if err == nil {
		s.GRPC.Port = p
	}

	// Set Metrics Port
	s.initializeMetricServerConfig(c)

	// If Tracing is set, Set tracing
	enableTracing(c, logger)

	initializeDataStores(c, logger, gofr)

	initializeNotifiers(c, gofr)

	s.GRPC.server = NewGRPCServer()

	return gofr
}

func (s *server) initializeMetricServerConfig(c Config) {
	// Set Metrics Port
	if val, err := strconv.Atoi(c.Get("METRICS_PORT")); err == nil && val >= 0 {
		s.MetricsPort = val
	}

	if route := c.Get("METRICS_ROUTE"); route != "" {
		s.MetricsRoute = "/" + strings.TrimPrefix(route, "/")
	}
}

func initializePubSub(c Config, logger log.Logger, k *Gofr) {
	pubsubBackend := c.Get("PUBSUB_BACKEND")
	if pubsubBackend == "" {
		return
	}

	switch strings.ToLower(pubsubBackend) {
	case datastore.Kafka, datastore.Avro:
		initializeKafka(c, k)
	case datastore.EventHub:
		initializeEventhub(c, k)
	case datastore.EventBridge:
		initializeEventBridge(c, logger, k)
	case datastore.GooglePubSub:
		initializeGooglePubSub(c, k)
	}
}

// InitializePubSubFromConfigs initialize pubsub object using the configuration provided
func InitializePubSubFromConfigs(c Config, l log.Logger, prefix string) (pubsub.PublisherSubscriber, error) {
	if prefix != "" {
		prefix += "_"
	}

	pubsubBackend := c.Get(prefix + "PUBSUB_BACKEND")
	if pubsubBackend == "" {
		return nil, errors.DataStoreNotInitialized{DBName: "PubSub", Reason: "pubsub backend not provided"}
	}

	switch strings.ToLower(pubsubBackend) {
	case datastore.Kafka, datastore.Avro:
		return initializeKafkaFromConfigs(c, l, prefix)
	case datastore.EventHub:
		return initializeEventhubFromConfigs(c, prefix)
	case datastore.EventBridge:
		return initializeEventBridgeFromConfigs(c, l, prefix)
	}

	return nil, errors.DataStoreNotInitialized{DBName: "Pubsub", Reason: "invalid pubsub backend"}
}

// initializeAvro initializes avro schema registry along with
// pubsub present in k.Pubsub, only if registryURL is present,
// else k.PubSub remains as is, either Kafka/Eventhub
func initializeAvro(c *avro.Config, k *Gofr) {
	pubsubKafka, _ := k.PubSub.(*kafka.Kafka)
	pubsubEventhub, _ := k.PubSub.(*eventhub.Eventhub)

	if pubsubKafka == nil && pubsubEventhub == nil {
		k.Logger.Error("Kafka/Eventhub not present, cannot use Avro")
		return
	}

	if c == nil {
		return
	}

	if c.URL == "" {
		k.Logger.Error("Schema registry URL is required for Avro")
	}

	ps, err := avro.NewWithConfig(c, k.PubSub)
	if err != nil {
		k.Logger.Errorf("Avro could not be initialized! SchemaRegistry: %v SchemaVersion: %v, Subject: %v, Error: %v",
			c.URL, c.Version, c.Subject, err)
	}

	if ps != nil {
		k.PubSub = ps
		k.Logger.Infof("Avro initialized! SchemaRegistry: %v SchemaVersion: %v, Subject: %v",
			c.URL, c.Version, c.Subject)
	}
}

// InitializeAvroFromConfigs initializes avro
func initializeAvroFromConfigs(c *avro.Config, ps pubsub.PublisherSubscriber) (pubsub.PublisherSubscriber, error) {
	pubsubKafka, _ := ps.(*kafka.Kafka)
	pubsubEventhub, _ := ps.(*eventhub.Eventhub)

	if pubsubKafka == nil && pubsubEventhub == nil {
		return nil, errors.DataStoreNotInitialized{DBName: "Avro", Reason: "Kafka/Eventhub not provided"}
	}

	return avro.NewWithConfig(c, ps)
}

// NewCMD creates a new gofr CMD application instance
func NewCMD() *Gofr {
	c := config.NewGoDotEnvProvider(log.NewLogger(), getConfigFolder())
	gofr := NewWithConfig(c)
	cmdApp := &cmdApp{Router: NewCMDRouter()}

	gofr.cmd = cmdApp
	cmdApp.server = gofr.Server

	go func() {
		const pushDuration = 10

		for {
			middleware.PushSystemStats()

			time.Sleep(time.Second * pushDuration)
		}
	}()

	cmdApp.context = NewContext(&responder.CMD{Logger: gofr.Logger}, request.NewCMDRequest(), gofr)

	tracer := otel.Tracer("gofr")

	// Start tracing span
	ctx, _ := tracer.Start(context.Background(), "CMD")

	cmdApp.context.Context = ctx

	return gofr
}

func enableTracing(c Config, logger log.Logger) {
	// If Tracing is set, initialize tracing
	if c.Get("TRACER_URL") == "" && c.Get("TRACER_EXPORTER") == "" && c.Get("GCP_PROJECT_ID") == "" {
		return
	}

	err := tracerProvider(c, logger)
	if err != nil {
		logger.Errorf("tracing is not enabled. Error %v", err)

		return
	}

	logger.Infof("tracing is enabled on: %v", c.Get("TRACER_URL"))
}

// initializeDataStores initializes the Gofr struct with all the data stores for which
// correct config is set in the environment
func initializeDataStores(c Config, logger log.Logger, k *Gofr) {
	// Redis
	initializeRedis(c, k)

	// DB
	initializeDB(c, k)

	// Cassandra
	initializeCassandra(c, k)

	// Mongo DB
	initializeMongoDB(c, k)

	// PubSub
	initializePubSub(c, logger, k)

	// Elasticsearch
	initializeElasticsearch(c, k)

	// Solr
	initializeSolr(c, k)

	// DynamoDB
	initializeDynamoDB(c, k)
}

func initializeDynamoDB(c Config, k *Gofr) {
	cfg := dynamoDBConfigFromEnv(c, "")

	if cfg.SecretAccessKey != "" && cfg.AccessKeyID != "" {
		var err error

		k.DynamoDB, err = datastore.NewDynamoDB(k.Logger, cfg)
		k.DatabaseHealth = append(k.DatabaseHealth, k.DynamoDBHealthCheck)

		if err != nil {
			k.Logger.Errorf("DynamoDB could not be initialized, error: %v\n", err)

			go dynamoRetry(cfg, k)

			return
		}

		k.Logger.Infof("DynamoDB initialized at %v", cfg.Endpoint)
	}
}

// InitializeDynamoDBFromConfig initializes DynamoDB
func InitializeDynamoDBFromConfig(c Config, l log.Logger, prefix string) (datastore.DynamoDB, error) {
	cfg := dynamoDBConfigFromEnv(c, prefix)
	return datastore.NewDynamoDB(l, cfg)
}

// initializeRedis initializes the Redis client in the Gofr struct if the Redis configuration is set
// in the environment, in case of an error, it logs the error
func initializeRedis(c Config, k *Gofr) {
	rc := redisConfigFromEnv(c, "")

	if rc.HostName != "" || rc.Port != "" {
		var err error

		k.Redis, err = datastore.NewRedis(k.Logger, rc)
		k.DatabaseHealth = append(k.DatabaseHealth, k.RedisHealthCheck)

		if err != nil {
			k.Logger.Errorf("could not connect to Redis, HostName: %s, Port: %s, error: %v\n",
				rc.HostName, rc.Port, err)

			go redisRetry(&rc, k)

			return
		}

		k.Logger.Infof("Redis connected. HostName: %s, Port: %s", rc.HostName, rc.Port)
	}
}

// InitializeRedisFromConfigs initializes redis
func InitializeRedisFromConfigs(c Config, l log.Logger, prefix string) (datastore.Redis, error) {
	cfg := redisConfigFromEnv(c, prefix)
	return datastore.NewRedis(l, cfg)
}

// initializeDB initializes the ORM object in the Gofr struct if the DB configuration is set
// in the environment, in case of an error, it logs the error
//
//nolint:gocognit //breaks code readability
func initializeDB(c Config, k *Gofr) {
	if c.Get("DB_HOST") != "" && c.Get("DB_PORT") != "" {
		dc := sqlDBConfigFromEnv(c, "")

		if strings.EqualFold(dc.ORM, "SQLX") {
			db, err := datastore.NewSQLX(dc)
			k.SetORM(db)

			k.DatabaseHealth = append(k.DatabaseHealth, k.SQLXHealthCheck)

			if err != nil {
				k.Logger.Errorf("could not connect to DB, HOST: %s, PORT: %s, Dialect: %s, error: %v\n",
					dc.HostName, dc.Port, dc.Dialect, err)

				go sqlxRetry(dc, k)

				return
			}

			k.Logger.Infof("DB connected, HostName: %s, Port: %s, Database: %s", dc.HostName, dc.Port, dc.Database)

			return
		}

		db, err := datastore.NewORM(dc)
		k.SetORM(db)

		k.DatabaseHealth = append(k.DatabaseHealth, k.SQLHealthCheck)

		if err != nil {
			k.Logger.Errorf("could not connect to DB, HostName: %s, Port: %s, Dialect: %s, error: %v\n",
				dc.HostName, dc.Port, dc.Dialect, err)

			go ormRetry(dc, k)

			return
		}

		k.Logger.Infof("DB connected, HostName: %s, Port: %s, Database: %s", dc.HostName, dc.Port, dc.Database)
	}
}

// InitializeGORMFromConfigs initializes GORM
func InitializeGORMFromConfigs(c Config, prefix string) (datastore.GORMClient, error) {
	cfg := sqlDBConfigFromEnv(c, prefix)
	return datastore.NewORM(cfg)
}

// InitializeSQLFromConfigs initializes SQL
func InitializeSQLFromConfigs(c Config, prefix string) (*datastore.SQLClient, error) {
	cfg := sqlDBConfigFromEnv(c, prefix)

	client, err := datastore.NewORM(cfg)
	if err != nil {
		return nil, err
	}

	var ds datastore.DataStore

	ds.SetORM(client)

	sqlClient := ds.DB()
	if sqlClient == nil {
		return nil, errors.DataStoreNotInitialized{DBName: "SQL"}
	}

	return sqlClient, nil
}

func initializeMongoDB(c Config, k *Gofr) {
	hostName := c.Get("MONGO_DB_HOST")
	port := c.Get("MONGO_DB_PORT")

	if hostName != "" && port != "" {
		mongoConfig := mongoDBConfigFromEnv(c, "")

		var err error

		k.MongoDB, err = datastore.GetNewMongoDB(k.Logger, mongoConfig)
		k.DatabaseHealth = append(k.DatabaseHealth, k.MongoHealthCheck)

		if err != nil {
			k.Logger.Errorf("could not connect to mongoDB, HOST: %s, PORT: %v, Error: %v\n", hostName, port, err)

			go mongoRetry(mongoConfig, k)

			return
		}

		k.Logger.Infof("MongoDB connected. HostName: %s, Port: %s, Database: %s", mongoConfig.HostName, mongoConfig.Port, mongoConfig.Database)
	}
}

// InitializeMongoDBFromConfigs initializes MongoDB
func InitializeMongoDBFromConfigs(c Config, l log.Logger, prefix string) (datastore.MongoDB, error) {
	cfg := mongoDBConfigFromEnv(c, prefix)
	return datastore.GetNewMongoDB(l, cfg)
}

func initializeKafka(c Config, k *Gofr) {
	hosts := c.Get("KAFKA_HOSTS")
	topic := c.Get("KAFKA_TOPIC")

	if hosts != "" && topic != "" {
		var err error

		kafkaConfig := kafkaConfigFromEnv(c, "")
		avroConfig := avroConfigFromEnv(c, "")

		k.PubSub, err = kafka.New(kafkaConfig, k.Logger)
		k.DatabaseHealth = append(k.DatabaseHealth, k.PubSubHealthCheck)

		if err != nil {
			k.Logger.Errorf("Kafka could not be initialized, Hosts: %v, Topic: %v, error: %v\n",
				hosts, topic, err)

			go kafkaRetry(kafkaConfig, avroConfig, k)

			return
		}

		k.Logger.Infof("Kafka initialized. Hosts: %v, Topic: %v\n", hosts, topic)

		// initialize Avro using Kafka pubsub if the schema url is specified
		if avroConfig.URL != "" {
			initializeAvro(avroConfig, k)
		}
	}
}

// initializeKafkaFromConfigs initializes kafka
func initializeKafkaFromConfigs(c Config, l log.Logger, prefix string) (pubsub.PublisherSubscriber, error) {
	cfg := kafkaConfigFromEnv(c, prefix)

	k, err := kafka.New(cfg, l)
	if err != nil {
		return nil, err
	}

	avroCfg := avroConfigFromEnv(c, prefix)
	if avroCfg != nil && avroCfg.URL != "" {
		return initializeAvroFromConfigs(avroCfg, k)
	}

	return k, nil
}

func initializeEventBridge(c Config, l log.Logger, k *Gofr) {
	if c.Get("EVENT_BRIDGE_BUS") != "" {
		cfg := eventbridgeConfigFromEnv(c, l, "")

		var err error

		k.PubSub, err = eventbridge.New(cfg)
		if err != nil {
			k.Logger.Errorf("AWS EventBridge could not be initialized, error: %v\n", err)

			go eventbridgeRetry(cfg, k)

			return
		}

		k.Logger.Info("AWS EventBridge initialized successfully")
	}
}

// InitializeEventBridgeFromConfigs initializes eventbridge
func initializeEventBridgeFromConfigs(c Config, l log.Logger, prefix string) (*eventbridge.Client, error) {
	cfg := eventbridgeConfigFromEnv(c, l, prefix)
	return eventbridge.New(cfg)
}

func initializeEventhub(c Config, k *Gofr) {
	hosts := c.Get("EVENTHUB_NAMESPACE")
	topic := c.Get("EVENTHUB_NAME")

	if hosts != "" && topic != "" {
		var err error

		avroConfig := avroConfigFromEnv(c, "")
		eventhubConfig := eventhubConfigFromEnv(c, "")

		k.PubSub, err = eventhub.New(&eventhubConfig)
		k.DatabaseHealth = append(k.DatabaseHealth, k.PubSubHealthCheck)

		if err != nil {
			k.Logger.Errorf("Azure Eventhub could not be initialized, Namespace: %v, Eventhub: %v, error: %v\n",
				hosts, topic, err)

			go eventhubRetry(&eventhubConfig, avroConfig, k)

			return
		}

		k.Logger.Infof("Azure Eventhub initialized, Namespace: %v, Eventhub: %v\n", hosts, topic)

		// initialize Avro using eventhub pubsub if the schema url is specified
		if avroConfig.URL != "" {
			initializeAvro(avroConfig, k)
		}
	}
}

// InitializeEventhubFromConfigs initializes eventhub
func initializeEventhubFromConfigs(c Config, prefix string) (pubsub.PublisherSubscriber, error) {
	cfg := eventhubConfigFromEnv(c, prefix)
	avroCfg := avroConfigFromEnv(c, prefix)

	e, err := eventhub.New(&cfg)
	if err != nil {
		return nil, err
	}

	if avroCfg != nil && avroCfg.URL != "" {
		return initializeAvroFromConfigs(avroCfg, e)
	}

	return e, nil
}

// initializeCassandra initializes the Cassandra/ YCQL client in the Gofr struct if the Cassandra configuration is set
// in the environment, in case of an error, it logs the error
//
//nolint:gocognit // reducing the function further is not required
func initializeCassandra(c Config, k *Gofr) {
	validDialects := map[string]bool{
		"cassandra": true,
		"ycql":      true,
	}

	host := c.Get("CASS_DB_HOST")
	port := c.Get("CASS_DB_PORT")
	dialect := strings.ToLower(c.Get("CASS_DB_DIALECT"))

	if host == "" || port == "" {
		return
	}

	if dialect == "" {
		dialect = "cassandra"
	}

	// Checks if dialect is valid
	if _, ok := validDialects[dialect]; !ok {
		k.Logger.Errorf("invalid dialect: supported dialects are - cassandra, ycql")
		return
	}

	var err error

	switch dialect {
	case "ycql":
		ycqlconfig := getYcqlConfigs(c, "")

		k.YCQL, err = datastore.GetNewYCQL(k.Logger, &ycqlconfig)
		k.DatabaseHealth = append(k.DatabaseHealth, k.YCQLHealthCheck)

		if err != nil {
			k.Logger.Errorf("could not connect to YCQL, Hosts: %s, Port: %s, Error: %v\n", host, port, err)

			go yclRetry(&ycqlconfig, k)

			return
		}

	default:
		cassandraCfg := cassandraConfigFromEnv(c, "")

		k.Cassandra, err = datastore.GetNewCassandra(k.Logger, cassandraCfg)
		k.DatabaseHealth = append(k.DatabaseHealth, k.CQLHealthCheck)

		if err != nil {
			k.Logger.Errorf("could not connect to Cassandra, Hosts: %s, Port: %s, Error: %v\n", host, port, err)

			go cassandraRetry(cassandraCfg, k)

			return
		}
	}
}

// InitializeCassandraFromConfigs initializes Cassandra
func InitializeCassandraFromConfigs(c Config, l log.Logger, prefix string) (datastore.Cassandra, error) {
	cfg := cassandraConfigFromEnv(c, prefix)
	return datastore.GetNewCassandra(l, cfg)
}

// InitializeYCQLFromConfigs initializes YCQL
func InitializeYCQLFromConfigs(c Config, l log.Logger, prefix string) (datastore.YCQL, error) {
	cfg := getYcqlConfigs(c, prefix)
	return datastore.GetNewYCQL(l, &cfg)
}

func getYcqlConfigs(c Config, prefix string) datastore.CassandraCfg {
	if prefix != "" {
		prefix += "_"
	}

	timeout, err := strconv.Atoi(c.Get(prefix + "CASS_DB_TIMEOUT"))
	if err != nil {
		// setting default timeout of 600 milliseconds
		timeout = 600
	}

	cassandraConnTimeout, err := strconv.Atoi(c.Get(prefix + "CASS_DB_CONN_TIMEOUT"))
	if err != nil {
		// setting default timeout of 600 milliseconds
		cassandraConnTimeout = 600
	}

	port, err := strconv.Atoi(c.Get(prefix + "CASS_DB_PORT"))
	if err != nil || port == 0 {
		// if any error, setting default
		port = 9042
	}

	return datastore.CassandraCfg{
		Hosts:               c.Get(prefix + "CASS_DB_HOST"),
		Port:                port,
		Username:            c.Get(prefix + "CASS_DB_USER"),
		Password:            c.Get(prefix + "CASS_DB_PASS"),
		Keyspace:            c.Get(prefix + "CASS_DB_KEYSPACE"),
		Timeout:             timeout,
		ConnectTimeout:      cassandraConnTimeout,
		ConnRetryDuration:   getRetryDuration(c.Get(prefix + "CASS_CONN_RETRY")),
		CertificateFile:     c.Get(prefix + "CASS_DB_CERTIFICATE_FILE"),
		KeyFile:             c.Get(prefix + "CASS_DB_KEY_FILE"),
		RootCertificateFile: c.Get(prefix + "CASS_DB_ROOT_CERTIFICATE_FILE"),
		HostVerification:    getBool(c.Get(prefix + "CASS_DB_HOST_VERIFICATION")),
		InsecureSkipVerify:  getBool(c.Get(prefix + "CASS_DB_INSECURE_SKIP_VERIFY")),
		DataCenter:          c.Get(prefix + "DATA_CENTER"),
	}
}

func initializeElasticsearch(c Config, k *Gofr) {
	elasticSearchCfg := elasticSearchConfigFromEnv(c, "")

	if (elasticSearchCfg.Host == "" || len(elasticSearchCfg.Ports) == 0) && elasticSearchCfg.CloudID == "" {
		return
	}

	var err error

	k.Elasticsearch, err = datastore.NewElasticsearchClient(k.Logger, &elasticSearchCfg)
	k.DatabaseHealth = append(k.DatabaseHealth, k.ElasticsearchHealthCheck)

	if err != nil {
		k.Logger.Errorf("could not connect to elasticsearch, HOST: %s, PORT: %v, Error: %v\n", elasticSearchCfg.Host, elasticSearchCfg.Ports, err)

		go elasticSearchRetry(&elasticSearchCfg, k)

		return
	}

	k.Logger.Infof("connected to elasticsearch, HOST: %s, PORT: %v\n", elasticSearchCfg.Host, elasticSearchCfg.Ports)
}

// InitializeElasticSearchFromConfigs initializes Elasticsearch
func InitializeElasticSearchFromConfigs(c Config, l log.Logger, prefix string) (datastore.Elasticsearch, error) {
	cfg := elasticSearchConfigFromEnv(c, prefix)
	return datastore.NewElasticsearchClient(l, &cfg)
}

func initializeSolr(c Config, k *Gofr) {
	host := c.Get("SOLR_HOST")
	port := c.Get("SOLR_PORT")

	if host == "" || port == "" {
		return
	}

	k.Solr = datastore.NewSolrClient(host, port)
	k.Logger.Infof("Solr connected. Host: %s, Port: %s \n", host, port)
}

// InitializeSolrFromConfigs initializes Solr
func InitializeSolrFromConfigs(c Config, prefix string) (datastore.Client, error) {
	if prefix != "" {
		prefix += "_"
	}

	host := c.Get(prefix + "SOLR_HOST")
	port := c.Get(prefix + "SOLR_PORT")

	if host == "" || port == "" {
		return datastore.Client{}, errors.DataStoreNotInitialized{DBName: "Solr", Reason: "Empty host"}
	}

	return datastore.NewSolrClient(host, port), nil
}

func initializeNotifiers(c Config, k *Gofr) {
	notifierBackend := c.Get("NOTIFIER_BACKEND")

	if notifierBackend == "" {
		return
	}

	if notifierBackend == "SNS" {
		initializeAwsSNS(c, k)
	}
}
func initializeAwsSNS(c Config, k *Gofr) {
	awsConfig := awsSNSConfigFromEnv(c, "")

	var err error

	k.Notifier, err = awssns.New(&awsConfig)
	k.DatabaseHealth = append(k.DatabaseHealth, k.Notifier.HealthCheck)

	if err != nil {
		k.Logger.Errorf("AWS SNS could not be initialized, error: %v\n", err)

		go awsSNSRetry(&awsConfig, k)

		return
	}

	k.Logger.Infof("AWS SNS initialized")
}

// InitializeAWSSNSFromConfigs initializes aws sns
func InitializeAWSSNSFromConfigs(c Config, prefix string) (notifier.Notifier, error) {
	awsConfig := awsSNSConfigFromEnv(c, prefix)
	return awssns.New(&awsConfig)
}

func getConfigFolder() (configFolder string) {
	if _, err := os.Stat("./configs"); err == nil {
		configFolder = "./configs"
	} else if _, err = os.Stat("../configs"); err == nil {
		configFolder = "../configs"
	} else {
		configFolder = "../../configs"
	}

	return
}

func initializeGooglePubSub(c Config, k *Gofr) {
	var err error

	googlePubSubConfigs := googlePubSubConfigFromEnv(c, "")

	k.PubSub, err = google.New(&googlePubSubConfigs, k.Logger)
	k.DatabaseHealth = append(k.DatabaseHealth, k.PubSubHealthCheck)

	if err != nil {
		k.Logger.Errorf("Cannot connect to google pubsub: %v", err)

		go googlePubsubRetry(googlePubSubConfigs, k)

		return
	}

	k.Logger.Infof("Google PubSub initialized")
}
