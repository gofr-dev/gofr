package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K20220329122459 struct {
}

func (k K20220329122459) Up(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration Up:20220329122459_customers_employee_update_columns")

	_, err := d.DB().Exec(AddCountry)
	if err != nil {
		return err
	}

	return nil
}

func (k K20220329122459) Down(d *datastore.DataStore, logger log.Logger) error {
	logger.Infof("Running migration Down:20220329122459_customers_employee_update_columns")

	_, err := d.DB().Exec(DropCountry)
	if err != nil {
		return err
	}

	return nil
}
