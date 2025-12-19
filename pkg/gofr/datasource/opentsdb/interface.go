package opentsdb

import (
	"context"
	"net"
	"net/http"
	"time"
)

//nolint:unused // connection interface defines all the methods to mock the connection returned while healthcheck implementation.
type connection interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// httpClient is an interface that wraps the http.Client's Do method.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Response defines the common behaviors all the specific response for
// different rest-apis should obey.
// Currently, it is an abstraction used in Client.sendRequest()
// to stored the different kinds of response contents for all the rest-apis.
type response interface {
	// getCustomParser can be used to retrieve a custom-defined parser.
	// Returning nil means current specific Response instance doesn't
	// need a custom-defined parse process, and just uses the default
	// json unmarshal method to parse the contents of the http response.
	getCustomParser(Logger) func(respCnt []byte) error
}

// Logger interface is used by opentsdb package to log information about request execution.
type Logger interface {
	Debug(args ...any)
	Debugf(pattern string, args ...any)
	Logf(pattern string, args ...any)
	Log(args ...any)
	Errorf(pattern string, args ...any)
	Fatal(args ...any)
}

type Metrics interface {
	NewCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)

	IncrementCounter(ctx context.Context, name string, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
}
