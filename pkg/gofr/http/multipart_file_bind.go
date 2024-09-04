package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"

	"gofr.dev/pkg/gofr/file"
)

var (
	errUnsupportedInterfaceType = errors.New("unsupported interface value type")
	errDataLengthExceeded       = errors.New("data length exceeds array capacity")
	errUnsupportedKind          = errors.New("unsupported kind")
	errSettingValueFailure      = errors.New("error setting value at index")
	errNotAStruct               = errors.New("provided value is not a struct")
	errFieldNotFound            = errors.New("field not found in struct")
	errUnexportedField          = errors.New("cannot set field; it might be unexported")
	errUnsupportedFieldType     = errors.New("unsupported type for field")
	errFieldsNotSet             = errors.New("no fields were set")
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
	value = dereferencePointerType(value)

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
	case reflect.Slice, reflect.Array:
		return uf.setSliceOrArrayValue(value, data)
	case reflect.Interface:
		return uf.setInterfaceValue(value, data)
	case reflect.Struct:
		return uf.setStructValue(value, data)
	case reflect.Invalid, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func,
		reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		return false, nil
	}

	return false, nil
}

func dereferencePointerType(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			// Initialize the pointer to a new value if it's nil
			value.Set(reflect.New(value.Type().Elem()))
		}

		value = value.Elem() // Dereference the pointer
	}

	return value
}

func (*formData) setInterfaceValue(value reflect.Value, data any) (bool, error) {
	if !value.CanSet() {
		return false, fmt.Errorf("%w: %s", errUnsupportedInterfaceType, value.Kind())
	}

	value.Set(reflect.ValueOf(data))

	return true, nil
}

func (uf *formData) setSliceOrArrayValue(value reflect.Value, data string) (bool, error) {
	elemType := value.Type().Elem()

	elements := strings.Split(data, ",")

	// Create a new slice/array with appropriate length and capacity
	var newSlice reflect.Value

	switch value.Kind() {
	case reflect.Slice:
		newSlice = reflect.MakeSlice(value.Type(), len(elements), len(elements))
	case reflect.Array:
		if len(elements) > value.Len() {
			return false, errDataLengthExceeded
		}

		newSlice = reflect.New(value.Type()).Elem()
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.String,
		reflect.Struct, reflect.UnsafePointer, reflect.Pointer:
		return false, fmt.Errorf("%w: %s", errUnsupportedKind, value.Kind())
	default:
		return false, fmt.Errorf("%w: %s", errUnsupportedKind, value.Kind())
	}

	// Create a reusable element value to avoid unnecessary allocations
	elemValue := reflect.New(elemType).Elem()

	// Set the elements of the slice/array
	for i, strVal := range elements {
		// Update the reusable element value
		if _, err := uf.setFieldValue(elemValue, strVal); err != nil {
			return false, fmt.Errorf("%w %d: %w", errSettingValueFailure, i, err)
		}

		newSlice.Index(i).Set(elemValue)
	}

	value.Set(newSlice)

	return true, nil
}

func (*formData) setStructValue(value reflect.Value, data string) (bool, error) {
	if value.Kind() != reflect.Struct {
		return false, errNotAStruct
	}

	dataMap, err := parseStringToMap(data)
	if err != nil {
		return false, err
	}

	for key, val := range dataMap {
		field, err := getFieldByName(value, key)
		if err != nil {
			return false, err
		}

		if err := setFieldValueFromData(field, val); err != nil {
			return false, err
		}

		// Return true and nil error once a field is set
		return true, nil
	}

	// Return false and an error if no fields were set
	return false, errFieldsNotSet
}

// getFieldByName retrieves a field by its name, considering case insensitivity.
func getFieldByName(value reflect.Value, key string) (reflect.Value, error) {
	field := value.FieldByName(key)
	if !field.IsValid() {
		field = findFieldByNameIgnoreCase(value, key)
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("%w: %s", errFieldNotFound, key)
		}
	}

	if !field.CanSet() {
		return reflect.Value{}, fmt.Errorf("%w: %s", errUnexportedField, key)
	}

	return field, nil
}

// setFieldValueFromData sets the field's value based on the provided data.
func setFieldValueFromData(field reflect.Value, data interface{}) error {
	switch val := data.(type) {
	case string:
		field.SetString(val)
	case int:
		field.SetInt(int64(val))
	case float64:
		field.SetFloat(val)
	case bool:
		field.SetBool(val)
	default:
		return fmt.Errorf("%w: %s, %T", errUnsupportedFieldType, field.Type().Name(), val)
	}

	return nil
}

type customUnmarshaller struct {
	dataMap map[string]interface{}
}

// UnmarshalJSON is a custom unmarshaller because json package in Go unmarshal numbers to float64 by default.
func (c *customUnmarshaller) UnmarshalJSON(data []byte) error {
	var rawData map[string]interface{}

	err := json.Unmarshal(data, &rawData)
	if err != nil {
		return err
	}

	dataMap := make(map[string]any, len(rawData))

	for key, val := range rawData {
		if valFloat, ok := val.(float64); ok {
			valInt := int(valFloat)
			if valFloat == float64(valInt) {
				val = valInt
			}
		}

		dataMap[key] = val
	}

	*c = customUnmarshaller{dataMap}

	return nil
}

func parseStringToMap(data string) (map[string]interface{}, error) {
	var c customUnmarshaller
	err := json.Unmarshal([]byte(data), &c)

	return c.dataMap, err
}

// Helper function to find a struct field by name, ignoring case.
func findFieldByNameIgnoreCase(value reflect.Value, name string) reflect.Value {
	t := value.Type()

	for i := 0; i < t.NumField(); i++ {
		if strings.EqualFold(t.Field(i).Name, name) {
			return value.Field(i)
		}
	}

	return reflect.Value{}
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
