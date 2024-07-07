package main

import (
	"errors"
	"fmt"

	"gofr.dev/examples/using-migrations/migrations"
	"gofr.dev/pkg/gofr"
)

const (
	queryGetEmployee    = "SELECT id,name,gender,contact_number,dob from employee where name = ?"
	queryInsertEmployee = "INSERT INTO employee (id, name, gender, contact_number,dob) values (?, ?, ?, ?, ?)"
)

func main() {
	// Create a new application
	a := gofr.New()

	// Add migrations to run
	a.Migrate(migrations.All())

	// Add all the routes
	a.GET("/employee", GetHandler)
	a.POST("/employee", PostHandler)

	// Run the application
	a.Run()
}

type Employee struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Phone  int    `json:"contact_number"`
	DOB    string `json:"dob"`
}

// GetHandler handles GET requests for retrieving employee information
func GetHandler(c *gofr.Context) (interface{}, error) {
	name := c.Param("name")
	if name == "" {
		return nil, errors.New("name can't be empty")
	}

	var emp Employee

	err := c.SQL.QueryRowContext(c, queryGetEmployee, name).
		Scan(&emp.ID, &emp.Name, &emp.Gender, &emp.Phone, &emp.DOB)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("DB Error: %v", err))
	}

	return emp, nil
}

// PostHandler handles POST requests for creating new employees
func PostHandler(c *gofr.Context) (interface{}, error) {
	var emp Employee
	if err := c.Bind(&emp); err != nil {
		c.Logger.Errorf("error in binding: %v", err)
		return nil, errors.New("invalid body")
	}

	// Execute the INSERT query
	_, err := c.SQL.ExecContext(c, queryInsertEmployee, emp.ID, emp.Name, emp.Gender, emp.Phone, emp.DOB)
	if err != nil {
		return Employee{}, errors.New(fmt.Sprintf("DB Error: %v", err))
	}

	return fmt.Sprintf("successfully posted entity: %v", emp.Name), nil
}
