package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20220329123813 struct {
}

func (k K20220329123813) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration up: 20220329123813_customers_employee_add_not_null_column")

	_, err := d.DB().Exec(AddNotNullColumn)
	if err != nil {
		return err
	}

	return nil
}

func (k K20220329123813) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration down: 20220329123813_customers_employee_add_not_null_column")

	_, err := d.DB().Exec(DeleteNotNullColumn)
	if err != nil {
		return err
	}

	return nil
}
