package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20230518180017 struct {
}

func (k K20230518180017) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migrations up:20220329122401_customers_employee_create_table.go")

	_, err := d.DB().Exec(DeleteNotNullColumn)
	if err != nil {
		return err
	}

	return nil
}

func (k K20230518180017) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migrations down:20220329122401_customers_employee_create_table.go")

	_, err := d.DB().Exec(AddNotNullColumn)
	if err != nil {
		return err
	}

	return nil
}
