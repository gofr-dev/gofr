package migrations

import (
	"context"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20231218131933 struct {
}

func (k K20231218131933) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running Migration Up:20231218131933_create_table")

	err := d.ClickHouse.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY,
    name varchar(50) ,
    age varchar(50)
) ENGINE = MergeTree ORDER BY id;`)
	if err != nil {
		return err
	}

	return nil
}

func (k K20231218131933) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running Migration Down:20231218131933_create_table")

	err := d.ClickHouse.Exec(context.Background(), `DROP TABLE users;`)
	if err != nil {
		return err
	}

	return nil
}
