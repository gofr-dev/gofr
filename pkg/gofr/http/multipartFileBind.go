package http

import (
	"fmt"
	"io"
	"mime/multipart"
	"reflect"
	"strconv"

	"gofr.dev/pkg/gofr/file"
)

type formData struct {
	fields map[string][]string
	files  map[string][]*multipart.FileHeader
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
	tag, ok := getFieldName(field)
	if !ok {
		return false, nil
	}

	if header, ok := uf.files[tag]; ok {
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

	if values, ok := uf.fields[tag]; ok {
		// handle non-file fields
		kind := value.Kind()
		data := values[0]

		switch kind {
		case reflect.String:
			value.SetString(data)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i, err := strconv.ParseInt(data, 10, 64)
			if err != nil {
				return false, err
			}

			value.SetInt(i)
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(data, 64)
			if err != nil {
				return false, err
			}

			value.SetFloat(f)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(values[0])
			if err != nil {
				return false, err
			}

			value.SetBool(boolVal)
		default:
			return false, fmt.Errorf("unsupported type for field %s: %v", field.Name, kind)
		}

		return true, nil
	}

	return false, nil
}

func getFieldName(field *reflect.StructField) (string, bool) {
	var (
		formTag = "form"
		fileTag = "file"
		key     string
	)

	if field.Tag.Get(formTag) != "" && field.IsExported() {
		key = field.Tag.Get(formTag)
	} else if field.Tag.Get(fileTag) != "" && field.IsExported() {
		key = field.Tag.Get(fileTag)
	} else if field.IsExported() {
		key = field.Name
	} else {
		return "", false
	}

	if key == "" || key == "-" {
		return "", false
	}

	return key, true
}
