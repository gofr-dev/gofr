package http

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

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

	if len(dataMap) == 0 {
		// Return false and an error if no fields were set
		return false, errFieldsNotSet
	}

	var (
		key string
		val any
	)

	for key, val = range dataMap {
		// we only need to iterate to get one element
		break
	}

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
