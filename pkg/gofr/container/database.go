package container

import (
	"database/sql"
	"fmt"
)

// dbConfig has those members which are necessary variables while connecting to database.
type dbConfig struct {
	HostName string
	User     string
	Password string
	Port     string
	Database string
}

func newMYSQL(config *dbConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local&interpolateParams=true",
		config.User,
		config.Password,
		config.HostName,
		config.Port,
		config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
