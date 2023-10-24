package store

import (
	"gofr.dev/examples/using-mysql/models"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type employee struct{}

// New is factory function for store layer
func New() Store {
	return employee{}
}

func (c employee) Get(ctx *gofr.Context) ([]models.Employee, error) {
	rows, err := ctx.DB().QueryContext(ctx, "SELECT id,name,email,phone,city FROM employees")
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	defer rows.Close()

	employees := make([]models.Employee, 0)

	for rows.Next() {
		var c models.Employee

		err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.City)
		if err != nil {
			return nil, errors.DB{Err: err}
		}

		employees = append(employees, c)
	}

	err = rows.Err()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return employees, nil
}

func (c employee) Create(ctx *gofr.Context, emp models.Employee) (models.Employee, error) {
	var resp models.Employee

	queryInsert := "INSERT INTO employees (id, name, email, phone, city) VALUES (?, ?, ?, ?, ?)"

	// Execute the INSERT query
	result, err := ctx.DB().ExecContext(ctx, queryInsert, emp.ID, emp.Name, emp.Email, emp.Phone, emp.City)

	if err != nil {
		return models.Employee{}, errors.DB{Err: err}
	}

	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return models.Employee{}, errors.DB{Err: err}
	}

	// Now, use a separate SELECT query to fetch the inserted data
	querySelect := "SELECT id, name, email, phone, city FROM employees WHERE id = ?"

	// Use QueryRowContext to execute the SELECT query and get a single row result
	err = ctx.DB().QueryRowContext(ctx, querySelect, lastInsertID).
		Scan(&resp.ID, &resp.Name, &resp.Email, &resp.Phone, &resp.City)

	// Handle the error if any
	if err != nil {
		return models.Employee{}, errors.DB{Err: err}
	}

	return resp, nil
}
