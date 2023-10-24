package store

import (
	"database/sql"

	"github.com/google/uuid"

	"gofr.dev/examples/using-postgres/model"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type customer struct{}

// New is factory function for store layer
func New() Store {
	return customer{}
}

type Store interface {
	Get(ctx *gofr.Context) ([]model.Customer, error)
	GetByID(ctx *gofr.Context, id uuid.UUID) (model.Customer, error)
	Update(ctx *gofr.Context, customer model.Customer) (model.Customer, error)
	Create(ctx *gofr.Context, customer model.Customer) (model.Customer, error)
	Delete(ctx *gofr.Context, id uuid.UUID) error
}

func (c customer) Get(ctx *gofr.Context) ([]model.Customer, error) {
	rows, err := ctx.DB().QueryContext(ctx, "SELECT id,name,email,phone FROM customers")
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	defer rows.Close()

	customers := make([]model.Customer, 0)

	for rows.Next() {
		var c model.Customer

		err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
		if err != nil {
			return nil, errors.DB{Err: err}
		}

		customers = append(customers, c)
	}

	err = rows.Err()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return customers, nil
}

func (c customer) GetByID(ctx *gofr.Context, id uuid.UUID) (model.Customer, error) {
	var resp model.Customer

	err := ctx.DB().QueryRowContext(ctx, " SELECT id,name,email,phone FROM customers where id=$1", id).
		Scan(&resp.ID, &resp.Name, &resp.Email, &resp.Phone)
	if err == sql.ErrNoRows {
		return model.Customer{}, errors.EntityNotFound{Entity: "customer", ID: id.String()}
	}

	return resp, nil
}

func (c customer) Update(ctx *gofr.Context, cust model.Customer) (model.Customer, error) {
	_, err := ctx.DB().ExecContext(ctx, "UPDATE customers SET name=$1,email=$2,phone=$3 WHERE id=$4",
		cust.Name, cust.Email, cust.Phone, cust.ID)
	if err != nil {
		return model.Customer{}, errors.DB{Err: err}
	}

	return cust, nil
}

func (c customer) Create(ctx *gofr.Context, cust model.Customer) (model.Customer, error) {
	var resp model.Customer

	uid := uuid.New()

	err := ctx.DB().QueryRowContext(ctx, "INSERT INTO customers(id,name,email,phone) VALUES($1,$2,$3,$4)"+
		" RETURNING  name,email,phone", uid, cust.Name, cust.Email, cust.Phone).Scan(
		&resp.Name, &resp.Email, &resp.Phone,
	)
	resp.ID = uid

	if err != nil {
		return model.Customer{}, errors.DB{Err: err}
	}

	return resp, nil
}

func (c customer) Delete(ctx *gofr.Context, id uuid.UUID) error {
	_, err := ctx.DB().ExecContext(ctx, "DELETE FROM customers where id=$1", id)
	if err != nil {
		return errors.DB{Err: err}
	}

	return nil
}
