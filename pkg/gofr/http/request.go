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
	errNoFileFound      = errors.New("no files were bounded")
	errIncompatibleType = errors.New("incompatible file type")
	errNonPointerBind   = errors.New("bind error, cannot bind to a non pointer type")
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
	}

	return nil
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

type formData struct {
	files map[string][]*multipart.FileHeader
}

func (r *Request) bindMultipart(ptr any) error {
	ptrVal := reflect.ValueOf(ptr)
	if ptrVal.Kind() == reflect.Ptr {
		ptrVal = ptrVal.Elem()
	} else {
		return errNonPointerBind
	}

	if err := r.req.ParseMultipartForm(defaultMaxMemory); err != nil {
		return err
	}

	fd := formData{files: r.req.MultipartForm.File}

	ok, err := fd.mapStruct(ptrVal, &reflect.StructField{})
	if err != nil {
		return err
	}

	if !ok {
		return errNoFileFound
	}

	return nil
}

func (uf *formData) mapStruct(val reflect.Value, field *reflect.StructField) (bool, error) {
	vKind := val.Kind()

	if vKind == reflect.Pointer {
		var isNew bool

		vPtr := val

		if val.IsNil() {
			isNew = true
			vPtr = reflect.New(val.Type().Elem())
		}

		ok, err := uf.mapStruct(vPtr.Elem(), field)
		if err != nil {
			return false, err
		}

		if isNew && ok {
			val.Set(vPtr)
		}

		return ok, nil
	}

	if vKind != reflect.Struct || !field.Anonymous {
		set, err := uf.trySet(val, field)
		if err != nil {
			return false, err
		}

		if set {
			return true, nil
		}
	}

	if vKind == reflect.Struct {
		var set bool

		tVal := val.Type()

		for i := 0; i < val.NumField(); i++ {
			sf := tVal.Field(i)
			if sf.PkgPath != "" && sf.Anonymous {
				continue
			}

			ok, err := uf.mapStruct(val.Field(i), &sf)
			if err != nil {
				return false, err
			}

			set = set || ok
		}

		return set, nil
	}

	return false, nil
}

func (uf *formData) trySet(value reflect.Value, field *reflect.StructField) (bool, error) {
	tag, ok := getFileName(field)
	if !ok {
		return false, nil
	}

	header, ok := uf.files[tag]
	if !ok {
		return false, nil
	}

	f, err := header[0].Open()
	if err != nil {
		return false, err
	}

	content, err := io.ReadAll(f)
	if err != nil {
		return false, err
	}

	switch {
	case value.Type() == reflect.TypeOf(file.Zip{}):
		zip, err := file.GenerateFile(content)
		if err != nil {
			return false, err
		}

		if value.Kind() == reflect.Ptr {
			value.Set(reflect.ValueOf(zip))
		} else {
			value.Set(reflect.ValueOf(*zip))
		}
	default:
		return false, errIncompatibleType
	}

	return true, nil
}

func getFileName(field *reflect.StructField) (string, bool) {
	var (
		tag = "file"
		key string
	)

	if field.Tag.Get(tag) == "-" {
		return "", false
	}

	if field.Tag.Get(tag) == "" {
		key = field.Name
	} else {
		key = field.Tag.Get(tag)
	}

	if key == "" {
		return "", false
	}

	return key, true
}
