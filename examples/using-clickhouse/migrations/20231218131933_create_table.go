package migrations

import (
	"context"

	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20231218131933 struct {
}

func (k K20231218131933) Up(d *datastore.DataStore, logger log.Logger) error {
	err := d.ClickHouse.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY,
    name varchar(50) ,
    age varchar(50)
) ENGINE = MergeTree ORDER BY id;`)

	return err
}

func (k K20231218131933) Down(d *datastore.DataStore, logger log.Logger) error {
	err := d.ClickHouse.Exec(context.Background(), `DROP TABLE users;`)

	return err
}
