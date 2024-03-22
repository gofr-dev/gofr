package http

import (
	"io"
	"mime/multipart"
	"reflect"

	"gofr.dev/pkg/gofr/file"
)

type formData struct {
	files map[string][]*multipart.FileHeader
}

func (uf *formData) mapStruct(val reflect.Value, field *reflect.StructField) (bool, error) {
	vKind := val.Kind()

	if vKind == reflect.Pointer {
		return uf.checkPointer(val, field)
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
		return uf.checkStruct(val)
	}

	return false, nil
}

func (uf *formData) checkPointer(val reflect.Value, field *reflect.StructField) (bool, error) {
	var (
		// isNew is a flag if the value of pointer is nil
		isNew bool
		vPtr  = val
	)

	if val.IsNil() {
		isNew = true

		// if the pointer is nil, assign an empty value
		vPtr = reflect.New(val.Type().Elem())
	}

	// try to set value with the underlying data type for the pointer
	ok, err := uf.mapStruct(vPtr.Elem(), field)
	if err != nil {
		return false, err
	}

	if isNew && ok {
		val.Set(vPtr)
	}

	return ok, nil
}

func (uf *formData) checkStruct(val reflect.Value) (bool, error) {
	var set bool

	tVal := val.Type()

	for i := 0; i < val.NumField(); i++ {
		sf := tVal.Field(i)
		if sf.PkgPath != "" && !sf.Anonymous {
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

	switch value.Interface().(type) {
	case file.Zip:
		zip, err := file.NewZip(content)
		if err != nil {
			return false, err
		}

		value.Set(reflect.ValueOf(*zip))
	case multipart.FileHeader:
		value.Set(reflect.ValueOf(*header[0]))
	default:
		return false, nil
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

	// we do not want to set unexported field
	if field.Tag.Get(tag) == "" && field.IsExported() {
		key = field.Name
	} else {
		key = field.Tag.Get(tag)
	}

	if key == "" {
		return "", false
	}

	return key, true
}
