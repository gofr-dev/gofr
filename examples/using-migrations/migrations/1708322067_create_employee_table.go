package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

const createTable = `CREATE TABLE IF NOT EXISTS employee
(
    id             int         not null
        primary key,
    name           varchar(50) not null,
    gender         varchar(6)  not null,
    contact_number varchar(10) not null
);`

const employee_date = `INSERT INTO employee (id, name, gender, contact_number) VALUES (1, 'Umang', "M", "0987654321");`

func createTableEmployee() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(createTable)
			if err != nil {
				return err
			}

			_, err = d.SQL.Exec(employee_date)
			if err != nil {
				return err
			}

			_, err = d.SQL.Exec("alter table employee add dob varchar(11) null;")
			if err != nil {
				return err
			}

			return nil
		},
	}
}
