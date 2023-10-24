/*
Package service provides generic HTTP client that can be used to call remote services.The service package also provides
support for custom headers, authentication, caching settings, and surge protection options.The package defines three main interfaces:
HTTP, SOAP, and Cacher, which can be used to implement various service clients.
*/
package service

import (
	"context"
	"time"
)

// HTTP is an interface for making HTTP requests and handling responses.
type HTTP interface {
	Get(ctx context.Context, api string, params map[string]interface{}) (*Response, error)
	Post(ctx context.Context, api string, params map[string]interface{}, body []byte) (*Response, error)
	Put(ctx context.Context, api string, params map[string]interface{}, body []byte) (*Response, error)
	Delete(ctx context.Context, api string, body []byte) (*Response, error)
	Patch(ctx context.Context, api string, params map[string]interface{}, body []byte) (*Response, error)

	GetWithHeaders(ctx context.Context, api string, params map[string]interface{}, headers map[string]string) (*Response, error)
	PostWithHeaders(ctx context.Context, api string, params map[string]interface{}, body []byte, headers map[string]string) (*Response, error)
	PutWithHeaders(ctx context.Context, api string, params map[string]interface{}, body []byte, headers map[string]string) (*Response, error)
	DeleteWithHeaders(ctx context.Context, api string, body []byte, headers map[string]string) (*Response, error)
	PatchWithHeaders(ctx context.Context, api string, params map[string]interface{}, body []byte, headers map[string]string) (*Response, error)

	Bind(resp []byte, i interface{}) error
	BindStrict(resp []byte, i interface{}) error

	// PropagateHeaders is used to specify the header keys that needs to be propagated through context
	// By default the headers: True-Client-IP, X-Zopsmart-Channel, X-Zopsmart-Location, X-Authenticated-UserId, X-Zopsmart-Tenant
	// are propagated.
	PropagateHeaders(headers ...string)

	// SetSurgeProtectorOptions sets the configuration for the surge protector, the default configuration is :-
	// surge protection is enabled, the heartbeat URL is /.well-known/heartbeat, retry frequency is 5 seconds.
	// The surge protector ensures that the HTTP client does not bombard a downstream service that is down,
	// it returns a 500 right away, until the service is back up again. It figures out if the service
	// is back up again by asynchronously making request to the heartbeat API until its up again
	SetSurgeProtectorOptions(isEnabled bool, customHeartbeatURL string, retryFrequencySeconds int)
}

// SOAP is an interface for making SOAP requests and handling responses.
type SOAP interface {
	Call(ctx context.Context, action string, body []byte) (*Response, error)
	CallWithHeaders(ctx context.Context, action string, body []byte, headers map[string]string) (*Response, error)
	Bind(resp []byte, i interface{}) error
	BindStrict(resp []byte, i interface{}) error
}

// Cacher is an interface for caching data.
type Cacher interface {
	Get(key string) ([]byte, error)
	Set(key string, content []byte, duration time.Duration) error
	Delete(key string) error
}
