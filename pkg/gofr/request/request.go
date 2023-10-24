package request

import (
	"net/http"
)

// Request provides the methods that are related to an incoming HTTP request
type Request interface {
	Request() *http.Request
	Params() map[string]string
	Param(string) string
	PathParam(string) string
	Bind(interface{}) error
	BindStrict(interface{}) error
	Header(string) string
	GetClaims() map[string]interface{}
	GetClaim(string) interface{}
}
