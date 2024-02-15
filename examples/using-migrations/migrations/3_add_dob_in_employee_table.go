package migrations

import "gofr.dev/pkg/gofr/migration"

func addDobInEmployeeTable() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.DB.Exec("alter table employee add dob varchar(11) null;")
			if err != nil {
				return err
			}

			return nil
		},
	}
}
