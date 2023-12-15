package gofr

import (
	"strconv"
	"time"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/datastore/pubsub/eventbridge"
	"gofr.dev/pkg/datastore/pubsub/eventhub"
	"gofr.dev/pkg/datastore/pubsub/google"
	"gofr.dev/pkg/datastore/pubsub/kafka"
	awssns "gofr.dev/pkg/notifier/aws-sns"
)

// awsSNSRetry retries connecting to aws SNS
// once connection is successful, retrying is terminated
func awsSNSRetry(c *awssns.Config, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying AWS SNS connection")

		var err error

		g.Notifier, err = awssns.New(c)
		if err == nil {
			g.Logger.Info("AWS SNS initialized successfully")

			break
		}
	}
}

// kafkaRetry retries connecting to kafka
// once connection is successful, retrying is terminated
func kafkaRetry(c *kafka.Config, avroConfig *avro.Config, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying Kafka connection")

		var err error

		g.PubSub, err = kafka.New(c, g.Logger)
		if err == nil {
			g.Logger.Info("Kafka initialized successfully")

			initializeAvro(avroConfig, g)

			break
		}
	}
}
func eventbridgeRetry(c *eventbridge.Config, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying AWS EventBridge connection")

		var err error

		g.PubSub, err = eventbridge.New(c)
		if err == nil {
			g.Logger.Info("AWS EventBridge initialized successfully")

			break
		}
	}
}

// eventhubRetry retries connecting to eventhub
// once connection is successful, retrying is terminated
// also while retrying to connect to eventhub, initializes avro as well if configs are set
func eventhubRetry(c *eventhub.Config, avroConfig *avro.Config, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying Eventhub connection")

		var err error

		g.PubSub, err = eventhub.New(c)
		if err == nil {
			g.Logger.Info("Eventhub initialized successfully, Namespace: %v, Eventhub: %v\\n", c.Namespace, c.EventhubName)

			initializeAvro(avroConfig, g)

			break
		}
	}
}

func mongoRetry(c *datastore.MongoConfig, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying MongoDB connection")

		var err error

		g.MongoDB, err = datastore.GetNewMongoDB(g.Logger, c)

		if err == nil {
			g.Logger.Info("MongoDB initialized successfully")

			break
		}
	}
}

func yclRetry(c *datastore.CassandraCfg, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying Ycql connection")

		var err error

		g.YCQL, err = datastore.GetNewYCQL(g.Logger, c)
		if err == nil {
			g.Logger.Info("Ycql initialized successfully")
			break
		}
	}
}

func cassandraRetry(c *datastore.CassandraCfg, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying Cassandra connection")

		var err error

		g.Cassandra, err = datastore.GetNewCassandra(g.Logger, c)
		if err == nil {
			g.Logger.Info("Cassandra initialized successfully")

			break
		}
	}
}

func ormRetry(c *datastore.DBConfig, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying ORM connection")

		db, err := datastore.NewORM(c)
		if err == nil {
			g.SetORM(db)
			g.Logger.Info("ORM initialized successfully")

			break
		}
	}
}

func sqlxRetry(c *datastore.DBConfig, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying SQLX connection")

		db, err := datastore.NewSQLX(c)
		if err == nil {
			g.SetORM(db)
			g.Logger.Info("SQLX initialized successfully")

			break
		}
	}
}

func redisRetry(c *datastore.RedisConfig, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnectionRetryDuration) * time.Second)
		g.Logger.Debug("Retrying Redis connection")

		var err error

		g.Redis, err = datastore.NewRedis(g.Logger, c)
		if err == nil {
			g.Logger.Info("Redis initialized successfully")

			break
		}
	}
}

func elasticSearchRetry(c *datastore.ElasticSearchCfg, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnectionRetryDuration) * time.Second)

		g.Logger.Debug("Retrying ElasticSearch connection")

		var err error

		g.Elasticsearch, err = datastore.NewElasticsearchClient(g.Logger, c)
		if err == nil {
			g.Logger.Infof("connected to elasticsearch, HOST: %s, PORT: %v\n", c.Host, c.Ports)

			break
		}
	}
}

func dynamoRetry(c datastore.DynamoDBConfig, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying DynamoDB connection")

		var err error

		g.DynamoDB, err = datastore.NewDynamoDB(g.Logger, c)
		if err == nil {
			g.Logger.Infof("DynamoDB initialized successfully, %v", c.Endpoint)

			break
		}
	}
}

func googlePubsubRetry(c google.Config, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying Google Pub-Sub connection")

		var err error

		g.PubSub, err = google.New(&c, g.Logger)
		if err == nil {
			g.Logger.Infof("Google Pub-Sub initialized successfully")

			break
		}
	}
}

func getRetryDuration(envDuration string) int {
	retryDuration, _ := strconv.Atoi(envDuration)
	if retryDuration == 0 {
		// default duration 30 seconds
		retryDuration = 30
	}

	return retryDuration
}

func getRedisDB(db string) int {
	DB, err := strconv.Atoi(db)
	if err != nil {
		return 0
	}

	return DB
}

func clickHouseRetry(c *datastore.ClickHouseConfig, g *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		g.Logger.Debug("Retrying ClickHouse connection")

		var err error

		g.ClickHouse, err = datastore.GetNewClickHouseDB(g.Logger, c)

		if err == nil {
			g.Logger.Info("ClickHouse initialized successfully")

			break
		}
	}
}
