package store

import (
	"github.com/google/uuid"

	"gofr.dev/examples/using-clickhouse/models"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type user struct{}

// New is factory function for store layer
func New() Store {
	return user{}
}

func (c user) Get(ctx *gofr.Context) ([]models.User, error) {
	rows, err := ctx.ClickHouse.Query(ctx, "SELECT id,name,age FROM users")
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	defer rows.Close()

	users := make([]models.User, 0)

	for rows.Next() {
		var c models.User

		err = rows.Scan(&c.ID, &c.Name, &c.Age)
		if err != nil {
			return nil, errors.DB{Err: err}
		}

		users = append(users, c)
	}

	err = rows.Err()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return users, nil
}

func (c user) GetByID(ctx *gofr.Context, id uuid.UUID) (models.User, error) {
	var u models.User

	err := ctx.ClickHouse.QueryRow(ctx, "SELECT * FROM users where id=?", id).Scan(&u.ID, &u.Name, &u.Age)

	if err != nil {
		return models.User{}, err
	}

	return u, nil
}

func (c user) Create(ctx *gofr.Context, user models.User) (models.User, error) {
	var resp models.User

	uid := uuid.New()

	queryInsert := "INSERT INTO users (id, name, age) VALUES (?, ?, ?)"

	// Execute the INSERT query
	err := ctx.ClickHouse.Exec(ctx, queryInsert, uid, user.Name, user.Age)
	if err != nil {
		return models.User{}, errors.DB{Err: err}
	}

	// Now, use a separate SELECT query to fetch the inserted data
	querySelect := "SELECT id, name, age FROM users WHERE id = ?"

	// Use QueryRowContext to execute the SELECT query and get a single row result
	err = ctx.ClickHouse.QueryRow(ctx, querySelect, uid).
		Scan(&resp.ID, &resp.Name, &resp.Age)

	// Handle the error if any
	if err != nil {
		return models.User{}, errors.DB{Err: err}
	}

	return resp, nil
}
