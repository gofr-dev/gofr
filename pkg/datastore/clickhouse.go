package datastore

import (
	"database/sql"
	"fmt"

	"github.com/ClickHouse/clickhouse-go"

	"gofr.dev/pkg"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

type ClickHouseConfig struct {
	Host              string
	Port              string
	Username          string
	Password          string
	ConnRetryDuration int
	MaxOpenConn       int
	MaxIdleConn       int
	MaxConnLife       int
}

type ClickHouseDB struct {
	*sql.DB
	config *ClickHouseConfig
	logger log.Logger
}

func getClickHouseConnectionString(config *ClickHouseConfig) string {
	return fmt.Sprintf("tcp://%s:%s?debug=true&username=%s&password=%s",
		config.Host, config.Port, config.Username, config.Password)
}

func GetNewClickHouseDB(logger log.Logger, config *ClickHouseConfig) (ClickHouseDB, error) {
	clickHouseConnectionString := getClickHouseConnectionString(config)

	connect, err := sql.Open("clickhouse", clickHouseConnectionString)
	if err != nil {
		return ClickHouseDB{}, err
	}

	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			logger.Errorf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			return ClickHouseDB{}, err
		}

		return ClickHouseDB{}, err
	}

	db := ClickHouseDB{DB: connect}

	return db, nil
}

// HealthCheck pings the clickHouse instance in gorm. If the ping does not return an error, the healthCheck status will be set to UP,
// else the healthCheck status will be DOWN
func (c ClickHouseDB) HealthCheck() types.Health {
	resp := types.Health{
		Name:   ClickHouse,
		Status: pkg.StatusDown,
		Host:   c.config.Host,
	}
	// The following check is for the condition when the connection to SQLX has not been made during initialization
	if c.DB == nil {
		return resp
	}

	err := c.DB.Ping()
	if err != nil {
		return resp
	}

	resp.Status = pkg.StatusUp
	resp.Details = c.DB.Stats()

	return resp
}
