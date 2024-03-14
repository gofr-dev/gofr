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
	errNonPointerBind         = errors.New("bind error, cannot bind to a non pointer type")
	errUnsupportedContentType = errors.New("unsupported content type")
	errIncompatibleType       = errors.New("incompatible file type")
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

	// Header has the properties of the file like name, size, etc. are present in Header
	Header *multipart.FileHeader
}

func (uf *File) Close() error {
	return uf.File.Close()
}

func (r *Request) bindMultipart(ptr any) error {
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
		if val.Field(i).Tag == "-" {
			continue
		}

		fileHeader, ok := getFileHeader(r.req.MultipartForm.File, val, i)
		if !ok {
			continue
		}

		field := reflect.ValueOf(ptr).Elem().Field(i)
		if !field.CanSet() {
			continue
		}

		f, err := fileHeader.Open()
		if err != nil {
			return err
		}

		err = trySet(f, fileHeader, field)
		if err != nil {
			return err
		}
	}

	return nil
}

func trySet(f multipart.File, header *multipart.FileHeader, value reflect.Value) error {
	content, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	contentType := http.DetectContentType(content)
	switch {
	case contentType == "application/zip" && value.Type() == reflect.TypeOf(&file.Zip{}):
		zip, err := file.NewZip(content)
		if err != nil {
			return err
		}

		value.Set(reflect.ValueOf(zip))
	case value.Type() == reflect.TypeOf(&File{}):
		value.Set(reflect.ValueOf(&File{f, header}))
	default:
		return errIncompatibleType
	}

	return nil
}

func getFileHeader(file map[string][]*multipart.FileHeader, val reflect.Type, i int) (*multipart.FileHeader, bool) {
	var (
		tag = "file"
		key string
	)

	if val.Field(i).Tag.Get(tag) == "-" {
		return nil, false
	}

	if val.Field(i).Tag.Get(tag) == "" {
		key = val.Field(i).Name
	} else {
		key = val.Field(i).Tag.Get(tag)
	}

	fileHeader, ok := file[key]
	if !ok {
		return nil, false
	}

	return fileHeader[0], true
}
