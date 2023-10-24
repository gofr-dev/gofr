package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

const CreateTableQuery = "CREATE TABLE IF NOT EXISTS persons (id int PRIMARY KEY, name text, age int, state text );"

type K20230113160630 struct {
}

func (k K20230113160630) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration up:20230113160630_createTable.go")

	err := d.Cassandra.Session.Query(CreateTableQuery).Exec()

	return err
}

func (k K20230113160630) Down(*datastore.DataStore, log.Logger) error {
	return nil
}
