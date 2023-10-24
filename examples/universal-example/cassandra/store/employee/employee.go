package employee

import (
	"strings"

	"gofr.dev/examples/universal-example/cassandra/entity"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type employee struct{}

//nolint:revive //employee should not get exposed
func New() employee {
	return employee{}
}

func (e employee) Get(ctx *gofr.Context, filter entity.Employee) []entity.Employee {
	var (
		employees []entity.Employee
		emp       entity.Employee
	)

	whereClause, values := getWhereClause(filter)

	query := "SELECT id, name, phone, email, city FROM employees" + whereClause
	item := ctx.Cassandra.Session.Query(query, values...).Iter()

	for item.Scan(&emp.ID, &emp.Name, &emp.Phone, &emp.Email, &emp.City) {
		employees = append(employees, entity.Employee{ID: emp.ID, Name: emp.Name, Phone: emp.Phone, Email: emp.Email, City: emp.City})
	}

	return employees
}

func (e employee) Create(ctx *gofr.Context, employee entity.Employee) ([]entity.Employee, error) {
	query := "INSERT INTO employees (id, name, phone, email, city) VALUES (?, ?, ?, ?, ?)"

	err := ctx.Cassandra.Session.Query(query, employee.ID, employee.Name, employee.Phone, employee.Email, employee.City).Exec()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return e.Get(ctx, entity.Employee{ID: employee.ID}), nil
}

func getWhereClause(e entity.Employee) (whereClause string, values []interface{}) {
	conditions := make([]string, 0)

	if e.ID != 0 {
		conditions = append(conditions, "id = ?")
		values = append(values, e.ID)
	}

	if e.Name != "" {
		conditions = append(conditions, "name = ?")
		values = append(values, e.Name)
	}

	if e.Phone != "" {
		conditions = append(conditions, "phone = ?")
		values = append(values, e.Phone)
	}

	if e.Email != "" {
		conditions = append(conditions, "email = ?")
		values = append(values, e.Email)
	}

	if e.City != "" {
		conditions = append(conditions, "city = ?")
		values = append(values, e.City)
	}

	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ") + " ALLOW FILTERING"
	}

	return whereClause, values
}
