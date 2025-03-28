package gofr

import "context"

// noopRequest is a non-operating implementation of Request interface
// this is required to prevent panics while executing cron jobs.
type noopRequest struct {
}

func (noopRequest) Context() context.Context {
	return context.Background()
}

func (noopRequest) Param(string) string {
	return ""
}

func (noopRequest) PathParam(string) string {
	return ""
}

func (noopRequest) HostName() string {
	return "gofr"
}

func (noopRequest) Bind(any) error {
	return nil
}

func (noopRequest) Params(string) []string {
	return nil
}

func (noopRequest) Header(string) string {
	return ""
}

func (noopRequest) GetClaims() any {
	return nil
}

func (noopRequest) GetClaim(string) any {
	return nil
}
