package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20220329123903 struct {
}

func (k K20220329123903) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration up: 20220329123903_customers_employee_alter_primary_key_test.go")

	_, err := d.DB().Exec(AlterPrimaryKey)
	if err != nil {
		return err
	}

	return nil
}

func (k K20220329123903) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration up: 20220329123903_customers_employee_alter_primary_key_test.go")

	_, err := d.DB().Exec(ResetPrimaryKey)
	if err != nil {
		return err
	}

	return nil
}
