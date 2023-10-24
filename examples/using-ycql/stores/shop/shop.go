package shop

import (
	"fmt"
	"strconv"
	"strings"

	"gofr.dev/examples/using-ycql/models"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type store struct{}

// New is a Factory function for Store layer
//
//nolint:revive // store should not be used without proper initilization with required dependency
func New() store {
	return store{}
}

func (s store) Get(ctx *gofr.Context, filter models.Shop) []models.Shop {
	var (
		shop  models.Shop
		shops []models.Shop
	)

	where, qp := getWhereClause(filter)
	q := ` select id, name, location ,state FROM shop ` + where
	iter := ctx.YCQL.Session.Query(q, qp...).Iter()

	for iter.Scan(&shop.ID, &shop.Name, &shop.Location, &shop.State) {
		shops = append(shops, models.Shop{ID: shop.ID, Name: shop.Name, Location: shop.Location, State: shop.State})
	}

	return shops
}

func (s store) Create(ctx *gofr.Context, data models.Shop) ([]models.Shop, error) {
	q := "INSERT INTO shop (id, name, location, state) VALUES (?, ?, ?, ?)"

	err := ctx.YCQL.Session.Query(q, data.ID, data.Name, data.Location, data.State).Exec()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return s.Get(ctx, models.Shop{ID: data.ID}), nil
}

func (s store) Delete(ctx *gofr.Context, id string) error {
	q := "DELETE FROM shop  WHERE id = ?"

	err := ctx.YCQL.Session.Query(q, id).Exec()
	if err != nil {
		return errors.DB{Err: err}
	}

	return err
}

func (s store) Update(ctx *gofr.Context, data models.Shop) ([]models.Shop, error) {
	q := "UPDATE shop"
	set, qp := genSetClause(&data)

	// No value is passed for update
	if qp == nil {
		return s.Get(ctx, models.Shop{ID: data.ID}), nil
	}

	q = fmt.Sprintf("%v %v where id = ?", q, set)
	id := strconv.Itoa(data.ID)

	qp = append(qp, id)

	err := ctx.YCQL.Session.Query(q, qp...).Exec()
	if err != nil {
		return nil, errors.DB{Err: err}
	}

	return s.Get(ctx, models.Shop{ID: data.ID}), nil
}

func genSetClause(s *models.Shop) (set string, qp []interface{}) {
	set = `SET`

	if s.Name != "" {
		set += " name = ?,"

		qp = append(qp, s.Name)
	}

	if s.Location != "" {
		set += " location = ?,"

		qp = append(qp, s.Location)
	}

	if s.State != "" {
		set += " state = ?,"

		qp = append(qp, s.State)
	}

	if set == "SET" {
		return "", nil
	}

	set = strings.TrimSuffix(set, ",")

	return set, qp
}

func getWhereClause(s models.Shop) (where string, qp []interface{}) {
	cond := make([]string, 0)

	if s.ID != 0 {
		cond = append(cond, "id = ?")
		qp = append(qp, s.ID)
	}

	if s.Name != "" {
		cond = append(cond, "name = ?")
		qp = append(qp, s.Name)
	}

	if s.Location != "" {
		cond = append(cond, "location = ?")
		qp = append(qp, s.Location)
	}

	if s.State != "" {
		cond = append(cond, "state = ?")
		qp = append(qp, s.State)
	}

	if len(cond) > 0 {
		where = " where " + strings.Join(cond, " AND ") + " ALLOW FILTERING"
	}

	return where, qp
}
