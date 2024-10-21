package opentsdb

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Conn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// HTTPClient is an interface that wraps the http.Client's Do method.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Response defines the common behaviors all the specific response for
// different rest-apis should obey.
// Currently, it is an abstraction used in OpentsdbClient.sendRequest()
// to stored the different kinds of response contents for all the rest-apis.
type Response interface {
	// SetStatus can be used to set the actual http status code of
	// the related http response for the specific Response instance
	SetStatus(code int)

	// GetCustomParser can be used to retrieve a custom-defined parser.
	// Returning nil means current specific Response instance doesn't
	// need a custom-defined parse process, and just uses the default
	// json unmarshal method to parse the contents of the http response.
	GetCustomParser() func(respCnt []byte) error

	// String returns the contents of the specific Response instance with
	// the string format
	String() string
}

// Logger interface is used by opentsdb package to log information about request execution.
type Logger interface {
	Debug(args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type Metrics interface {
	NewHistogram(name, desc string, buckets ...float64)

	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
