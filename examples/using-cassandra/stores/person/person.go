package person

import (
	"fmt"
	"strconv"
	"strings"

	"gofr.dev/examples/using-cassandra/models"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type store struct{}

// New is factory function for person store
//
//nolint:revive // person should not be used without proper initilization with required dependency
func New() store {
	return store{}
}

func (s store) Get(ctx *gofr.Context, filter models.Person) []models.Person {
	cassDB := ctx.Cassandra.Session
	whereCL, values := getWhereClause(filter)
	query := `SELECT id, name, age ,state FROM persons`
	querystring := query + " " + whereCL
	iter := cassDB.Query(querystring, values...).Iter()

	var (
		person  models.Person
		persons []models.Person
	)

	for iter.Scan(&person.ID, &person.Name, &person.Age, &person.State) {
		persons = append(persons, person)
	}

	return persons
}

func (s store) Create(ctx *gofr.Context, data models.Person) ([]models.Person, error) {
	cassDB := ctx.Cassandra.Session
	q := "INSERT INTO persons (id, name, age, state) VALUES (?, ?, ?, ?)"

	err := cassDB.Query(q, data.ID, data.Name, data.Age, data.State).Exec()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return s.Get(ctx, models.Person{ID: data.ID}), nil
}

func (s store) Delete(ctx *gofr.Context, id string) error {
	cassDB := ctx.Cassandra.Session
	q := "DELETE FROM persons WHERE id = ?"

	err := cassDB.Query(q, id).Exec()
	if err != nil {
		return errors.DB{Err: err}
	}

	return err
}

func (s store) Update(ctx *gofr.Context, data models.Person) ([]models.Person, error) {
	cassDB := ctx.Cassandra.Session
	q := "UPDATE persons"
	set, qp := genSetClause(data)

	// No value is passed for update
	if qp == nil {
		return s.Get(ctx, models.Person{ID: data.ID}), nil
	}

	q = fmt.Sprintf("%v %v WHERE id = ?", q, set)
	id := strconv.Itoa(data.ID)

	qp = append(qp, id)

	err := cassDB.Query(q, qp...).Exec()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return s.Get(ctx, models.Person{ID: data.ID}), nil
}

func genSetClause(p models.Person) (set string, qp []interface{}) {
	set = `SET`

	if p.Name != "" {
		set += " name = ?,"

		qp = append(qp, p.Name)
	}

	if p.Age > 0 {
		set += " age = ?,"

		qp = append(qp, p.Age)
	}

	if p.State != "" {
		set += " state = ?,"

		qp = append(qp, p.State)
	}

	if set == "SET" {
		return "", nil
	}

	set = strings.TrimSuffix(set, ",")

	return set, qp
}

func getWhereClause(p models.Person) (where string, qp []interface{}) {
	where = " WHERE "

	if p.ID != 0 {
		where += "id = ? AND "

		qp = append(qp, p.ID)
	}

	if p.Name != "" {
		where += "name = ? AND "

		qp = append(qp, p.Name)
	}

	if p.Age != 0 {
		where += "age = ? AND "

		qp = append(qp, p.Age)
	}

	if p.State != "" {
		where += "state = ? AND "

		qp = append(qp, p.State)
	}

	where = strings.TrimSuffix(where, "AND ")
	where = strings.TrimSuffix(where, " WHERE ")

	if where != "" {
		where += " ALLOW FILTERING "
	}

	return where, qp
}
