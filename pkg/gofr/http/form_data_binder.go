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
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return false, fmt.Errorf("%w: %s", errUnsupportedKind, value.Kind())
	}

	elemType := value.Type().Elem()

	elements := strings.Split(data, ",")

	// Create a new slice/array with appropriate length and capacity
	var newSlice reflect.Value

	if value.Kind() == reflect.Slice {
		newSlice = reflect.MakeSlice(value.Type(), len(elements), len(elements))
	} else if len(elements) > value.Len() {
		return false, errDataLengthExceeded
	} else {
		newSlice = reflect.New(value.Type()).Elem()
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
		return false, errFieldsNotSet
	}

	numFieldsSet := 0

	var multiErr error

	// Create a map for case-insensitive lookups
	caseInsensitiveMap := make(map[string]any)
	for key, val := range dataMap {
		caseInsensitiveMap[strings.ToLower(key)] = val
	}

	for i := 0; i < value.NumField(); i++ {
		fieldType := value.Type().Field(i)
		fieldValue := value.Field(i)
		fieldName := fieldType.Name

		// Perform case-insensitive lookup for the key in dataMap
		val, exists := caseInsensitiveMap[strings.ToLower(fieldName)]
		if !exists {
			continue
		}

		if !fieldValue.CanSet() {
			multiErr = fmt.Errorf("%w: %s", errUnexportedField, fieldName)
			continue
		}

		if err := setFieldValueFromData(fieldValue, val); err != nil {
			multiErr = fmt.Errorf("%w; %w", multiErr, err)
			continue
		}

		numFieldsSet++
	}

	if numFieldsSet == 0 {
		return false, errFieldsNotSet
	}

	return true, multiErr
}

// setFieldValueFromData sets the field's value based on the provided data.
func setFieldValueFromData(field reflect.Value, data any) error {
	switch field.Kind() {
	case reflect.String:
		return setStringField(field, data)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setIntField(field, data)
	case reflect.Float32, reflect.Float64:
		return setFloatField(field, data)
	case reflect.Bool:
		return setBoolField(field, data)
	case reflect.Invalid, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		return fmt.Errorf("%w: %s, %T", errUnsupportedFieldType, field.Type().Name(), data)
	default:
		return fmt.Errorf("%w: %s, %T", errUnsupportedFieldType, field.Type().Name(), data)
	}
}

type customUnmarshaller struct {
	dataMap map[string]any
}

// UnmarshalJSON is a custom unmarshaller because json package in Go unmarshal numbers to float64 by default.
func (c *customUnmarshaller) UnmarshalJSON(data []byte) error {
	var rawData map[string]any

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

func parseStringToMap(data string) (map[string]any, error) {
	var c customUnmarshaller
	err := json.Unmarshal([]byte(data), &c)

	return c.dataMap, err
}

func setStringField(field reflect.Value, data any) error {
	if val, ok := data.(string); ok {
		field.SetString(val)
		return nil
	}

	return fmt.Errorf("%w: expected string but got %T", errUnsupportedFieldType, data)
}

func setIntField(field reflect.Value, data any) error {
	if val, ok := data.(int); ok {
		field.SetInt(int64(val))
		return nil
	}

	return fmt.Errorf("%w: expected int but got %T", errUnsupportedFieldType, data)
}

func setFloatField(field reflect.Value, data any) error {
	if val, ok := data.(float64); ok {
		field.SetFloat(val)
		return nil
	}

	return fmt.Errorf("%w: expected float64 but got %T", errUnsupportedFieldType, data)
}

func setBoolField(field reflect.Value, data any) error {
	if val, ok := data.(bool); ok {
		field.SetBool(val)
		return nil
	}

	return fmt.Errorf("%w: expected bool but got %T", errUnsupportedFieldType, data)
}
