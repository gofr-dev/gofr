package user

import (
	"net/http"

	"gofr.dev/examples/using-http-service/models"
	"gofr.dev/examples/using-http-service/services"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

type service struct {
	svc services.HTTPService
}

// New is factory function for service layer
//
//nolint:revive // handler should not be used without proper initialization of the required dependency
func New(svc services.HTTPService) service {
	return service{svc: svc}
}

func (s service) Get(ctx *gofr.Context, name string) (models.User, error) {
	resp, err := s.svc.Get(ctx, "user/"+name, nil)
	if err != nil {
		return models.User{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return models.User{}, s.getErrorResponse(resp.Body, resp.StatusCode)
	}

	var u models.User

	data := struct {
		Data interface{} `json:"data"`
	}{Data: &u}

	err = s.bind(resp.Body, &data)
	if err != nil {
		return models.User{}, err
	}

	return u, nil
}

// getErrorResponse unmarshalls the error response and returns it.
func (s service) getErrorResponse(body []byte, statusCode int) error {
	resp := struct {
		Errors []errors.Response `json:"errors"`
	}{}

	if err := s.bind(body, &resp); err != nil {
		return err
	}

	err := errors.MultipleErrors{StatusCode: statusCode}
	for i := range resp.Errors {
		err.Errors = append(err.Errors, &resp.Errors[i])
	}

	return err
}

// bind unmarshalls response body to data and returns Bind Error if an error occurs.
func (s service) bind(body []byte, data interface{}) error {
	err := s.svc.Bind(body, data)
	if err != nil {
		return &errors.Response{
			Code:   "Bind Error",
			Reason: "failed to bind response",
			Detail: err,
		}
	}

	return nil
}
