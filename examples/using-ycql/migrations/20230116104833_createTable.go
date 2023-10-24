package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

const CreateTable = "CREATE TABLE IF NOT EXISTS shop" +
	" ( id int PRIMARY KEY,name varchar, location varchar ,state varchar ) WITH transactions = { 'enabled' : true };"

type K20230116104833 struct {
}

func (k K20230116104833) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migrations Up:20230116104833_createTable.go")

	err := d.YCQL.Session.Query(CreateTable).Exec()

	return err
}

func (k K20230116104833) Down(_ *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migrations Down:20230116104833_createTable.go")

	return nil
}
