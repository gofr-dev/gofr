package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20220329122659 struct {
}

func (k K20220329122659) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration Up:20220329122659_customers_employee_alter_column_datatype")

	_, err := d.DB().Exec(AlterType)
	if err != nil {
		return err
	}

	return nil
}

func (k K20220329122659) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration Up:20220329122659_customers_employee_alter_column_datatype")

	_, err := d.DB().Exec(ResetType)
	if err != nil {
		return err
	}

	return nil
}
