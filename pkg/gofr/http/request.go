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

	// The request media types Bind can decode; also the set a QUERY request may
	// carry (see ValidateQueryContentType).
	contentTypeJSON           = "application/json"
	contentTypeMultipartForm  = "multipart/form-data"
	contentTypeFormURLEncoded = "application/x-www-form-urlencoded"

	// "binary/octet-stream" is GoFr's own spelling, kept for compatibility;
	// "application/octet-stream" is the RFC 2046 one and is what clients
	// actually send. Both are accepted by Bind and ValidateQueryContentType.
	contentTypeBinary      = "binary/octet-stream"
	contentTypeOctetStream = "application/octet-stream"

	// MethodQuery is the HTTP QUERY method (RFC 10008). Go's net/http has no such
	// constant yet, so GoFr exports it here for use in routing, middleware, RBAC
	// method lists, and method comparisons.
	MethodQuery = "QUERY"
)

// bindersByMediaType is the single source of truth for which request bodies
// GoFr's Bind can decode, keyed by canonical media type. Bind's dispatch and
// isBindableMediaType both derive from this map, so ValidateQueryContentType
// and Bind cannot disagree on the supported set: a new media type is a new
// map entry and both paths pick it up at once.
//
// The "binary/octet-stream" GoFr spelling and the RFC 2046 "application/
// octet-stream" spelling both map to bindBinary — keeping the two spellings
// live is a deliberate compatibility choice, not a drift.
//
//nolint:gochecknoglobals // process-wide, read-only dispatch table for Bind.
var bindersByMediaType = map[string]func(*Request, any) error{
	contentTypeJSON:           (*Request).bindJSON,
	contentTypeMultipartForm:  (*Request).bindMultipart,
	contentTypeFormURLEncoded: (*Request).bindFormURLEncoded,
	contentTypeBinary:         (*Request).bindBinary,
	contentTypeOctetStream:    (*Request).bindBinary,
}

// isBindableMediaType reports whether ct is a media type Bind can decode.
// Reads from bindersByMediaType, so the answer is exactly the set Bind's
// dispatch can dispatch on. Callers must pass the media type in canonical
// form (lowercase, no parameters); use mediaType() to normalize before calling.
func isBindableMediaType(ct string) bool {
	_, ok := bindersByMediaType[ct]

	return ok
}

// ValidateQueryContentType enforces RFC 10008's requirement that a QUERY request
// carry a Content-Type the server can interpret. A missing Content-Type yields a
// 400 (ErrorMissingParam); a present-but-unsupported one yields a 415
// (ErrorUnsupportedMediaType). The supported set is isBindableMediaType, which
// reads bindersByMediaType — the same map Bind's dispatch reads, so what the
// guard accepts is exactly what Bind can decode. POST and the other verbs never
// call this.
//
// Normalization goes through mediaType() (mime.ParseMediaType), so casing and
// parameter differences — Application/JSON, application/json; charset=utf-8 —
// resolve to the same canonical form the predicate checks.
func ValidateQueryContentType(r *http.Request) error {
	raw := r.Header.Get("Content-Type")
	if strings.TrimSpace(raw) == "" {
		return ErrorMissingParam{Params: []string{"Content-Type"}}
	}

	ct := mediaType(raw)
	if !isBindableMediaType(ct) {
		return ErrorUnsupportedMediaType{ContentType: raw}
	}

	return nil
}

var (
	errNoFileFound    = errors.New("no files were bounded")
	errNonPointerBind = errors.New("bind error, cannot bind to a non pointer type")
	errNonSliceBind   = errors.New("bind error: input is not a pointer to a byte slice")
)

// Request is an abstraction over the underlying http.Request. This abstraction is useful because it allows us
// to create applications without being aware of the transport. cmd.Request is another such abstraction.
type Request struct {
	req        *http.Request
	pathParams map[string]string
}

// NewRequest creates a new GoFr Request instance from the given http.Request.
func NewRequest(r *http.Request) *Request {
	req := RequestFor(r)

	return &req
}

// RequestFor returns a Request by value.
//
// The per-request construction site stores the Request inside the request
// Context, which is itself heap allocated and has exactly the same lifetime.
// Returning a value lets the caller place it in that same allocation instead of
// taking a second one. Callers needing a pointer keep using NewRequest.
func RequestFor(r *http.Request) Request {
	return Request{
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

// Bind parses the request body and binds it to the provided interface. The
// media type → binder dispatch is bindersByMediaType, the same map
// isBindableMediaType reads — the two paths share one source, not two
// hand-maintained copies of the supported set.
func (r *Request) Bind(i any) error {
	// Binding into a non-pointer would unmarshal into a throwaway copy and leave
	// the caller's value untouched, so reject it up front instead of silently
	// doing nothing.
	if rv := reflect.ValueOf(i); rv.Kind() != reflect.Pointer {
		return errNonPointerBind
	}

	binder, ok := bindersByMediaType[mediaType(r.req.Header.Get("Content-Type"))]
	if !ok {
		// An unrecognized media type is a no-op: the target is left as it was
		// and no error is reported.
		//
		// This silently discards a body the caller probably meant to bind, and
		// an earlier revision of this PR rejected it with 415 for exactly that
		// reason. That is a breaking change and it is not a quiet one:
		// fetch(url, {method: "POST", body: str}) with no headers sends
		// text/plain, so a handler that returns the Bind error would start
		// answering 415 to a very common client shape. Those requests bind
		// nothing today, but services that do not need the body work, and they
		// would stop working. Left as it is deliberately.
		return nil
	}

	return binder(r, i)
}

// bindJSON decodes a JSON request body into i. Extracted so JSON dispatch
// matches the other binders in bindersByMediaType — every entry is a method
// on *Request with the same signature.
func (r *Request) bindJSON(i any) error {
	body, err := r.body()
	if err != nil {
		return err
	}

	return json.Unmarshal(body, &i)
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
