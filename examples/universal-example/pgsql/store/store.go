package store

import (
	"gofr.dev/examples/universal-example/pgsql/entity"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

// Store is the abstraction of core layer
type Store interface {
	Get(c *gofr.Context) ([]entity.Employee, error)
	Create(c *gofr.Context, employee entity.Employee) error
}

// Employee is a type on which all core layer's functionality is implemented
type employee struct{}

// New returns Employee core
//
//nolint:revive // employee should not be exposed
func New() employee {
	return employee{}
}

func (e employee) Get(c *gofr.Context) ([]entity.Employee, error) {
	employees := make([]entity.Employee, 0)

	rows, err := c.DB().Query("SELECT * FROM employees")
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	defer rows.Close()

	for rows.Next() {
		var employee entity.Employee

		err = rows.Scan(&employee.ID, &employee.Name, &employee.Phone, &employee.Email, &employee.City)
		if err != nil {
			return nil, errors.DB{Err: err}
		}

		employees = append(employees, employee)
	}

	err = rows.Err()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return employees, nil
}

func (e employee) Create(c *gofr.Context, employee entity.Employee) error {
	_, err := c.DB().Exec("INSERT INTO employees(id, name, phone, email, city) VALUES($1, $2, $3, $4, $5)",
		employee.ID, employee.Name, employee.Phone, employee.Email, employee.City)
	if err != nil {
		return errors.DB{Err: err}
	}

	return nil
}
