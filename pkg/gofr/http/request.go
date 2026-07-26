package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/gorilla/mux"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB

	contentTypeJSON = "application/json"
)

var (
	errNoFileFound    = errors.New("no files were bounded")
	errNonPointerBind = errors.New("bind error, cannot bind to a non pointer type")
	// errUnsupportedContentType is returned when a request carries a body whose
	// Content-Type Bind has no decoder for. Reported rather than ignored so the
	// body is never silently discarded.
	errUnsupportedContentType = errors.New("bind error, unsupported content type")
	errNonSliceBind           = errors.New("bind error: input is not a pointer to a byte slice")
)

// Request is an abstraction over the underlying http.Request. This abstraction is useful because it allows us
// to create applications without being aware of the transport. cmd.Request is another such abstraction.
type Request struct {
	req        *http.Request
	pathParams map[string]string
}

// NewRequest creates a new GoFr Request instance from the given http.Request.
func NewRequest(r *http.Request) *Request {
	return &Request{
		req:        r,
		pathParams: mux.Vars(r),
	}
}

// Param returns the query parameter with the given key.
func (r *Request) Param(key string) string {
	return r.req.URL.Query().Get(key)
}

// Context returns the context of the request.
func (r *Request) Context() context.Context {
	return r.req.Context()
}

// PathParam retrieves a path parameter from the request.
func (r *Request) PathParam(key string) string {
	return r.pathParams[key]
}

// Bind parses the request body and binds it to the provided interface.
func (r *Request) Bind(i any) error {
	// Binding into a non-pointer would unmarshal into a throwaway copy and leave
	// the caller's value untouched, so reject it up front instead of silently
	// doing nothing.
	if rv := reflect.ValueOf(i); rv.Kind() != reflect.Pointer {
		return errNonPointerBind
	}

	contentType := mediaType(r.req.Header.Get("Content-Type"))

	switch contentType {
	case contentTypeJSON:
		body, err := r.body()
		if err != nil {
			return err
		}

		return json.Unmarshal(body, &i)
	case "multipart/form-data":
		return r.bindMultipart(i)
	case "application/x-www-form-urlencoded":
		return r.bindFormURLEncoded(i)
	case "binary/octet-stream":
		return r.bindBinary(i)
	}

	// An unrecognized media type means the body cannot be decoded. Returning nil
	// would leave the caller's target zeroed with no error — the same silent
	// no-op the non-pointer check above rejects, and the likelier one in
	// practice (a client that posts a body but omits Content-Type).
	//
	// Only a request that actually carries a body is rejected: with no body
	// there is nothing to decode and nothing lost, so binding stays a no-op for
	// callers that Bind defensively on bodyless requests.
	body, err := r.body()
	if err != nil {
		return err
	}

	if len(body) > 0 {
		return fmt.Errorf("%w: %q", errUnsupportedContentType, contentType)
	}

	return nil
}

// mediaType extracts the bare media type from a Content-Type header value.
// Per RFC 9110 the media type is case-insensitive and may be followed by
// parameters and arbitrary optional whitespace, so `Application/JSON` and
// `application/json ; charset=utf-8` must both resolve to `application/json`.
func mediaType(header string) string {
	parsed, _, err := mime.ParseMediaType(header)
	if err != nil && parsed == "" {
		// The header is malformed beyond a bad parameter list; fall back to a
		// best-effort parse rather than losing an otherwise usable media type.
		return strings.ToLower(strings.TrimSpace(strings.Split(header, ";")[0]))
	}

	return parsed
}

// HostName retrieves the hostname from the request.
func (r *Request) HostName() string {
	proto := r.req.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}

	return fmt.Sprintf("%s://%s", proto, r.req.Host)
}

// Params returns the values associated with the given query parameter key. Each value is
// additionally split on commas, so ?tag=a,b&tag=c yields []string{"a", "b", "c"}.
//
// If the key is absent, a nil slice is returned. A handler may return this value directly, and a
// nil slice reaches the client as "null" in the response body where an empty one reaches it as "[]".
func (r *Request) Params(key string) []string {
	values := r.req.URL.Query()[key]

	// Deliberately not preallocated: that would return an empty slice for an absent key and
	// silently change the response body from `null` to `[]`. The hint is also a poor estimate,
	// since len(values) under-counts as soon as any value contains a comma.
	//nolint:prealloc // preserves the nil return for an absent key; see comment above
	var result []string

	for _, value := range values {
		result = append(result, strings.Split(value, ",")...)
	}

	return result
}

func (r *Request) body() ([]byte, error) {
	// A server-received request always has a non-nil Body, but one built by
	// hand — as in a handler unit test — may not, and io.ReadAll(nil) panics.
	// Treat an absent body as an empty one so callers get an ordinary decode
	// error (or, for a type with no decoder, a no-op) instead of a crash.
	if r.req.Body == nil {
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(r.req.Body)
	if err != nil {
		return nil, err
	}

	r.req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil
}

func (r *Request) bindMultipart(ptr any) error {
	return r.bindForm(ptr, true)
}

func (r *Request) bindFormURLEncoded(ptr any) error {
	return r.bindForm(ptr, false)
}

func (r *Request) bindForm(ptr any, isMultipart bool) error {
	ptrVal := reflect.ValueOf(ptr)
	if ptrVal.Kind() != reflect.Pointer {
		return errNonPointerBind
	}

	ptrVal = ptrVal.Elem()

	var fd formData

	if isMultipart {
		if err := r.req.ParseMultipartForm(defaultMaxMemory); err != nil {
			return err
		}

		fd = formData{files: r.req.MultipartForm.File, fields: r.req.MultipartForm.Value}
	} else {
		if err := r.req.ParseForm(); err != nil {
			return err
		}

		fd = formData{fields: r.req.Form}
	}

	ok, err := fd.mapStruct(ptrVal, nil)
	if err != nil {
		return err
	}

	if !ok {
		if isMultipart {
			return errNoFileFound
		}

		return errFieldsNotSet
	}

	return nil
}

// bindBinary handles binding for binary/octet-stream content type.
func (r *Request) bindBinary(raw any) error {
	// Ensure raw is a pointer to a byte slice
	byteSlicePtr, ok := raw.(*[]byte)
	if !ok {
		return fmt.Errorf("%w: %v", errNonSliceBind, raw)
	}

	body, err := r.body()
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Assign the body to the provided slice
	*byteSlicePtr = body

	return nil
}
