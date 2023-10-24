package service

import (
	"gofr.dev/pkg/gofr"
)

type svc struct {
	http HTTPService
}

//nolint:revive // svc should not be exposed
func New(service HTTPService) svc {
	return svc{http: service}
}

type Response struct {
	Data string `json:"data"`
}

func (s svc) Log(ctx *gofr.Context, serviceName string) (string, error) {
	resp, err := s.http.Get(ctx, "level", map[string]interface{}{"service": serviceName})
	if err != nil {
		return "", err
	}

	var r Response

	err = s.http.Bind(resp.Body, &r)
	if err != nil {
		return "", err
	}

	return r.Data, nil
}

func (s svc) Hello(ctx *gofr.Context, name string) (string, error) {
	resp, err := s.http.Get(ctx, "hello", map[string]interface{}{"name": name})
	if err != nil {
		return "", err
	}

	var r Response

	err = s.http.Bind(resp.Body, &r)
	if err != nil {
		return "", err
	}

	return r.Data, nil
}
