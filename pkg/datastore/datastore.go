package datastore

import (
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	"gorm.io/gorm"

	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

// DataStore represents a database connection pool for various types of databases
type DataStore struct {
	rdb  SQLClient
	gorm GORMClient
	sqlx SQLXClient

	ClickHouse ClickHouseDB

	Logger        log.Logger
	MongoDB       MongoDB
	Redis         Redis
	ORM           interface{}
	Cassandra     Cassandra
	YCQL          YCQL
	PubSub        pubsub.PublisherSubscriber
	Solr          Client
	Elasticsearch Elasticsearch
	DynamoDB      DynamoDB
}

// QueryLogger represents a structure to log database queries.
type QueryLogger struct {
	Hosts     string     `json:"host,omitempty"`
	Query     []string   `json:"query"`
	Duration  int64      `json:"duration"`
	StartTime time.Time  `json:"-"`
	Logger    log.Logger `json:"-"`
	DataStore string     `json:"datastore"`
}

// GORM returns a GORM database instance, initializing it if necessary, based on the DataStore's internal state and ORM interface.
func (ds *DataStore) GORM() *gorm.DB {
	var err error

	if ds.gorm.DB != nil {
		return ds.gorm.DB
	}

	if db, ok := ds.ORM.(GORMClient); ok {
		ds.gorm = db
		if db.DB != nil {
			ds.rdb.DB, err = db.DB.DB()
			if err != nil {
				ds.Logger.Warn(err)
				return db.DB
			}
		}

		return db.DB
	}

	if gormDB, ok := ds.ORM.(*gorm.DB); ok {
		return gormDB
	}

	return nil
}

// SQLX returns an initialized SQLX instance
func (ds *DataStore) SQLX() *sqlx.DB {
	if ds.sqlx.DB != nil {
		return ds.sqlx.DB
	}

	if db, ok := ds.ORM.(SQLXClient); ok {
		ds.sqlx = db
		if db.DB != nil {
			ds.rdb.DB = db.DB.DB
		}

		return db.DB
	}

	if sqlxDB, ok := ds.ORM.(*sqlx.DB); ok {
		return sqlxDB
	}

	return nil
}

// DB returns an initialized SQLClient instance
func (ds *DataStore) DB() *SQLClient {
	if ds.rdb.DB != nil {
		return &ds.rdb
	}

	if db := ds.GORM(); db != nil {
		dbg, err := ds.GORM().DB()
		if err != nil {
			ds.Logger.Warn(err)
			return &SQLClient{DB: nil, config: ds.gorm.config, logger: ds.Logger}
		}

		return &SQLClient{DB: dbg, config: ds.gorm.config, logger: ds.Logger}
	}

	if db := ds.SQLX(); db != nil {
		return &SQLClient{DB: ds.SQLX().DB, config: ds.sqlx.config, logger: ds.Logger}
	}

	if db, ok := ds.ORM.(*sql.DB); ok {
		ds.rdb.DB = db
		return &SQLClient{DB: db, config: ds.rdb.config, logger: ds.Logger}
	}

	return nil
}

// SetORM sets the ORM based on GORM or SQLX
func (ds *DataStore) SetORM(client interface{}) {
	// making sure that either gorm or sqlx is set and not both
	if ds.ORM != nil {
		return
	}

	switch v := client.(type) {
	case GORMClient:
		v.logger = ds.Logger
		ds.gorm = v

		if v.DB != nil {
			sqlDB, err := v.DB.DB()
			if err != nil {
				ds.Logger.Warn(err)
				return
			}

			ds.rdb.DB, ds.rdb.config, ds.rdb.logger = sqlDB, v.config, ds.Logger
			ds.ORM = v.DB
		}

	case SQLXClient:
		if v.DB != nil {
			ds.ORM = v.DB
		}

		ds.sqlx = v
	}
}

// SQLHealthCheck pings the sql instance. If the ping does not return an error, the healthCheck status will be set to UP,
// else the healthCheck status will be DOWN
func (ds *DataStore) SQLHealthCheck() types.Health {
	return ds.gorm.HealthCheck()
}

// SQLXHealthCheck pings the sqlx instance. If the ping does not return an error, the healthCheck status will be set to UP,
// else the healthCheck status will be DOWN.
func (ds *DataStore) SQLXHealthCheck() types.Health {
	return ds.sqlx.HealthCheck()
}

// CQLHealthCheck performs a query operation on the cql instance. If the operation does not return an error, the healthCheck
// status will be set to UP, else the healthCheck status will be DOWN.
func (ds *DataStore) CQLHealthCheck() types.Health {
	return ds.Cassandra.HealthCheck()
}

// YCQLHealthCheck performs a query operation on the ycql instance. If the operation does not return an error, the healthCheck
// status will be set to UP, else the healthCheck status will be DOWN.
func (ds *DataStore) YCQLHealthCheck() types.Health {
	return ds.YCQL.HealthCheck()
}

// ElasticsearchHealthCheck pings the Elasticsearch instance. If the ping does not return an error,
// the healthCheck status will be set to UP, else the healthCheck status will be DOWN
func (ds *DataStore) ElasticsearchHealthCheck() types.Health {
	return ds.Elasticsearch.HealthCheck()
}

// MongoHealthCheck pings the MongoDB instance. If the ping does not return an error,
// the healthCheck status will be set to UP, else the healthCheck status will be DOWN.
func (ds *DataStore) MongoHealthCheck() types.Health {
	return ds.MongoDB.HealthCheck()
}

// RedisHealthCheck pings the redis instance. If the ping does not return an error,
// the healthCheck status will be set to UP, else the healthCheck status will be DOWN
func (ds *DataStore) RedisHealthCheck() types.Health {
	return ds.Redis.HealthCheck()
}

// PubSubHealthCheck pings the pubsub instance. If the ping does not return an error,
// the healthCheck status will be set to UP, else the healthCheck status will be DOWN
func (ds *DataStore) PubSubHealthCheck() types.Health {
	return ds.PubSub.HealthCheck()
}

// DynamoDBHealthCheck executes a ListTable API operation. If the returned error is not nil,
// the healthCheck status will be set to DOWN, else the healthCheck status will be UP
func (ds *DataStore) DynamoDBHealthCheck() types.Health {
	return ds.DynamoDB.HealthCheck()
}

// ClickHouseHealthCheck pings the ClickHouse instance. If the ping does not return an error,
// the healthCheck status will be set to UP, else the healthCheck status will be DOWN.
func (ds *DataStore) ClickHouseHealthCheck() types.Health {
	return ds.ClickHouse.HealthCheck()
}
