package http

import (
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
		return uf.setFile(value, header)
	}

	if values, ok := uf.fields[tag]; ok {
		return uf.setFieldValue(value, values[0])
	}

	return false, nil
}

func (*formData) setFile(value reflect.Value, header []*multipart.FileHeader) (bool, error) {
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

func (uf *formData) setFieldValue(value reflect.Value, data string) (bool, error) {
	kind := value.Kind()
	switch kind {
	case reflect.String:
		return uf.setStringValue(value, data)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uf.setIntValue(value, data)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uf.setUintValue(value, data)
	case reflect.Float32, reflect.Float64:
		return uf.setFloatValue(value, data)
	case reflect.Bool:
		return uf.setBoolValue(value, data)
	case reflect.Invalid, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		// These types are not supported for setting via form data
		return false, nil
	default:
		return false, nil
	}
}

func (*formData) setStringValue(value reflect.Value, data string) (bool, error) {
	value.SetString(data)

	return true, nil
}

func (*formData) setIntValue(value reflect.Value, data string) (bool, error) {
	i, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return false, err
	}

	value.SetInt(i)

	return true, nil
}

func (*formData) setUintValue(value reflect.Value, data string) (bool, error) {
	ui, err := strconv.ParseUint(data, 10, 64)
	if err != nil {
		return false, err
	}

	value.SetUint(ui)

	return true, nil
}

func (*formData) setFloatValue(value reflect.Value, data string) (bool, error) {
	f, err := strconv.ParseFloat(data, 64)
	if err != nil {
		return false, err
	}

	value.SetFloat(f)

	return true, nil
}

func (*formData) setBoolValue(value reflect.Value, data string) (bool, error) {
	boolVal, err := strconv.ParseBool(data)
	if err != nil {
		return false, err
	}

	value.SetBool(boolVal)

	return true, nil
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
