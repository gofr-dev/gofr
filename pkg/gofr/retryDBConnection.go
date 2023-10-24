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
func awsSNSRetry(c *awssns.Config, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying AWS SNS connection")

		var err error

		k.Notifier, err = awssns.New(c)
		if err == nil {
			k.Logger.Info("AWS SNS initialized successfully")

			break
		}
	}
}

// kafkaRetry retries connecting to kafka
// once connection is successful, retrying is terminated
func kafkaRetry(c *kafka.Config, avroConfig *avro.Config, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying Kafka connection")

		var err error

		k.PubSub, err = kafka.New(c, k.Logger)
		if err == nil {
			k.Logger.Info("Kafka initialized successfully")

			initializeAvro(avroConfig, k)

			break
		}
	}
}
func eventbridgeRetry(c *eventbridge.Config, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying AWS EventBridge connection")

		var err error

		k.PubSub, err = eventbridge.New(c)
		if err == nil {
			k.Logger.Info("AWS EventBridge initialized successfully")

			break
		}
	}
}

// eventhubRetry retries connecting to eventhub
// once connection is successful, retrying is terminated
// also while retrying to connect to eventhub, initializes avro as well if configs are set
func eventhubRetry(c *eventhub.Config, avroConfig *avro.Config, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying Eventhub connection")

		var err error

		k.PubSub, err = eventhub.New(c)
		if err == nil {
			k.Logger.Info("Eventhub initialized successfully, Namespace: %v, Eventhub: %v\\n", c.Namespace, c.EventhubName)

			initializeAvro(avroConfig, k)

			break
		}
	}
}

func mongoRetry(c *datastore.MongoConfig, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying MongoDB connection")

		var err error

		k.MongoDB, err = datastore.GetNewMongoDB(k.Logger, c)

		if err == nil {
			k.Logger.Info("MongoDB initialized successfully")

			break
		}
	}
}

func yclRetry(c *datastore.CassandraCfg, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying Ycql connection")

		var err error

		k.YCQL, err = datastore.GetNewYCQL(k.Logger, c)
		if err == nil {
			k.Logger.Info("Ycql initialized successfully")
			break
		}
	}
}

func cassandraRetry(c *datastore.CassandraCfg, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying Cassandra connection")

		var err error

		k.Cassandra, err = datastore.GetNewCassandra(k.Logger, c)
		if err == nil {
			k.Logger.Info("Cassandra initialized successfully")

			break
		}
	}
}

func ormRetry(c *datastore.DBConfig, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying ORM connection")

		db, err := datastore.NewORM(c)
		if err == nil {
			k.SetORM(db)
			k.Logger.Info("ORM initialized successfully")

			break
		}
	}
}

func sqlxRetry(c *datastore.DBConfig, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying SQLX connection")

		db, err := datastore.NewSQLX(c)
		if err == nil {
			k.SetORM(db)
			k.Logger.Info("SQLX initialized successfully")

			break
		}
	}
}

func redisRetry(c *datastore.RedisConfig, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnectionRetryDuration) * time.Second)

		k.Logger.Debug("Retrying Redis connection")

		var err error

		k.Redis, err = datastore.NewRedis(k.Logger, *c)
		if err == nil {
			k.Logger.Info("Redis initialized successfully")

			break
		}
	}
}

func elasticSearchRetry(c *datastore.ElasticSearchCfg, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnectionRetryDuration) * time.Second)

		k.Logger.Debug("Retrying ElasticSearch connection")

		var err error

		k.Elasticsearch, err = datastore.NewElasticsearchClient(k.Logger, c)
		if err == nil {
			k.Logger.Infof("connected to elasticsearch, HOST: %s, PORT: %v\n", c.Host, c.Ports)

			break
		}
	}
}

func dynamoRetry(c datastore.DynamoDBConfig, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying DynamoDB connection")

		var err error

		k.DynamoDB, err = datastore.NewDynamoDB(k.Logger, c)
		if err == nil {
			k.Logger.Infof("DynamoDB initialized successfully, %v", c.Endpoint)

			break
		}
	}
}

func googlePubsubRetry(c google.Config, k *Gofr) {
	for {
		time.Sleep(time.Duration(c.ConnRetryDuration) * time.Second)

		k.Logger.Debug("Retrying Google Pub-Sub connection")

		var err error

		k.PubSub, err = google.New(&c, k.Logger)
		if err == nil {
			k.Logger.Infof("Google Pub-Sub initialized successfully")

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
