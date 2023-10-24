package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20220329122401 struct {
}

func (k K20220329122401) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running Migration Up:20220329122401_customers_employee_create_table")

	_, err := d.DB().Exec(CreateTable)
	if err != nil {
		return err
	}

	return nil
}

func (k K20220329122401) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running Migration Down:20220329122401_customers_employee_create_table")

	_, err := d.DB().Exec(DroopTable)
	if err != nil {
		return err
	}

	return nil
}
