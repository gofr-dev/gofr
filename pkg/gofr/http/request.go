package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"

	"gofr.dev/pkg/gofr/file"
)

const (
	defaultMaxMemory = 32 << 20 // 32 MB
)

var (
	errNonPointerBind         = errors.New("err")
	errUnsupportedContentType = errors.New("unsupported content type")
	errIncompatibleType       = errors.New("")
)

// Request is an abstraction over the underlying http.Request. This abstraction is useful because it allows us
// to create applications without being aware of the transport. cmd.Request is another such abstraction.
type Request struct {
	req        *http.Request
	pathParams map[string]string
}

func NewRequest(r *http.Request) *Request {
	return &Request{
		req:        r,
		pathParams: mux.Vars(r),
	}
}

func (r *Request) Param(key string) string {
	return r.req.URL.Query().Get(key)
}

func (r *Request) Context() context.Context {
	return r.req.Context()
}

func (r *Request) PathParam(key string) string {
	return r.pathParams[key]
}

func (r *Request) Bind(i interface{}) error {
	v := r.req.Header.Get("content-type")
	contentType := strings.Split(v, ";")[0]

	switch contentType {
	case "application/json":
		body, err := r.body()
		if err != nil {
			return err
		}

		return json.Unmarshal(body, &i)
	case "multipart/form-data":
		return r.bindMultipart(i)
	default:
		return errUnsupportedContentType
	}
}

func (r *Request) GetClaims() map[string]interface{} {
	claims, ok := r.Context().Value("JWTClaims").(jwt.MapClaims)
	if !ok {
		return nil
	}

	return claims
}

func (r *Request) HostName() string {
	proto := r.req.Header.Get("X-forwarded-proto")
	if proto == "" {
		proto = "http"
	}

	return fmt.Sprintf("%s://%s", proto, r.req.Host)
}

func (r *Request) body() ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.req.Body)
	if err != nil {
		return nil, err
	}

	r.req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return bodyBytes, nil
}

type File struct {
	// it embeds the file type that is present in the request
	multipart.File

	// Header has the properties of the file like name, size, content type are present in Header
	Header *multipart.FileHeader
}

func (uf *File) Close() error {
	return uf.File.Close()
}

func (r *Request) bindMultipart(ptr interface{}) error {
	vType := reflect.TypeOf(ptr)
	vKind := vType.Kind()

	if vKind != reflect.Pointer {
		return errNonPointerBind
	}

	if err := r.req.ParseMultipartForm(defaultMaxMemory); err != nil {
		return err
	}

	val := vType.Elem()

	for i := 0; i < val.NumField(); i++ {
		fileHeader, ok := r.req.MultipartForm.File[val.Field(i).Name]
		if !ok {
			continue
		}

		field := reflect.ValueOf(ptr).Elem().Field(i)
		if !field.CanSet() {
			continue
		}

		f, err := fileHeader[0].Open()
		if err != nil {
			return err
		}

		content, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		err = trySet(content, field)
		if err != nil {
			return err
		}
	}

	return nil
}

func trySet(content []byte, value reflect.Value) error {
	contentType := http.DetectContentType(content)
	switch contentType {
	case "application/zip":
		zip, err := file.NewZip(content)
		if err != nil {
			return err
		}

		if value.Type() == reflect.TypeOf(zip) {
			value.Set(reflect.ValueOf(zip))

			return nil
		}

		return errIncompatibleType
	default:
		return errIncompatibleType
	}
}
