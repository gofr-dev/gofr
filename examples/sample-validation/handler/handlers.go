package handler

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/types"
)

type details struct {
	Phone types.Phone `json:"phone"`
	Email types.Email `json:"email"`
}

func ValidateEntry(ctx *gofr.Context) (interface{}, error) {
	var detail details

	err := ctx.Bind(&detail)
	if err != nil {
		return nil, err
	}

	err = types.Validate(detail.Phone, detail.Email)
	if err != nil {
		return nil, err
	}

	return "successful validation", nil
}
