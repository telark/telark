package common

import (
	"fmt"
	"reflect"
	"strings"
)

func MapToJsonPayload(input any) (map[string]any, error) {
	if input == nil {
		return nil, nil
	}

	result := make(map[string]any)
	value := reflect.ValueOf(input)

	// Handle pointers
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}

	// Only process structs
	if value.Kind() != reflect.Struct {
		return result, nil
	}

	typeOfInput := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := typeOfInput.Field(i)

		// Parse JSON tag with options
		key, omitEmpty := parseJSONTag(fieldType.Tag.Get("json"))
		if key == "-" {
			continue // Skip ignored fields
		}

		// Handle omitempty for zero values
		if omitEmpty && isZeroValue(field) {
			continue
		}

		// Process field value
		fieldValue, err := processFieldValue(field)
		if err != nil {
			return nil, err
		}

		if fieldValue != nil {
			result[key] = fieldValue
		}
	}

	return result, nil
}

// parseJSONTag extracts field name and options from JSON tag
func parseJSONTag(tag string) (string, bool) {
	if tag == "" {
		return "", false
	}

	parts := strings.Split(tag, ",")
	fieldName := parts[0]

	omitEmpty := false
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) == "omitempty" {
			omitEmpty = true
			break
		}
	}

	return fieldName, omitEmpty
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return v.IsZero()
	default:
		return false
	}
}

func processFieldValue(field reflect.Value) (any, error) {
	switch field.Kind() {
	case reflect.Struct:
		return MapToJsonPayload(field.Interface())
	case reflect.Slice, reflect.Array:
		return processSliceValue(field)
	case reflect.Map:
		return processMapValue(field)
	case reflect.Ptr:
		if field.IsNil() {
			return nil, nil
		}
		return processFieldValue(field.Elem())
	case reflect.Interface:
		if field.IsNil() {
			return nil, nil
		}
		return field.Interface(), nil
	default:
		return field.Interface(), nil
	}
}

func processSliceValue(field reflect.Value) (any, error) {
	length := field.Len()
	if length == 0 {
		return []any{}, nil
	}

	slice := make([]any, 0, length)
	for i := 0; i < length; i++ {
		elem := field.Index(i)
		value, err := processFieldValue(elem)
		if err != nil {
			return nil, err
		}
		slice = append(slice, value)
	}

	return slice, nil
}

func processMapValue(field reflect.Value) (any, error) {
	if field.Len() == 0 {
		return map[string]any{}, nil
	}

	result := make(map[string]any)
	iter := field.MapRange()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Convert key to string
		keyStr, ok := key.Interface().(string)
		if !ok {
			keyStr = fmt.Sprintf("%v", key.Interface())
		}

		valueProcessed, err := processFieldValue(value)
		if err != nil {
			return nil, err
		}

		result[keyStr] = valueProcessed
	}

	return result, nil
}
