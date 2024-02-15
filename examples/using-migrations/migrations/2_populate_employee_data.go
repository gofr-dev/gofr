package migrations

import "gofr.dev/pkg/gofr/migration"

const employee_date = `INSERT INTO employee (id, name, gender, contact_number) VALUES (1, 'Umang', "M", "0987654321");`

func populateEmployeeData() migration.Migrate {
	return migration.Migrate{
		UP: func(m migration.Datasource) error {
			_, err := m.DB.Exec(employee_date)
			if err != nil {
				return err
			}

			return nil
		},
	}
}
