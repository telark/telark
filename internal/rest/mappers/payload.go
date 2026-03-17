package mappers

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/plsyro/rest/constants"
)

func MapToJSONPayload(input any) (map[string]any, error) {
	if input == nil {
		return nil, nil
	}

	result := make(map[string]any)
	value := reflect.ValueOf(input)

	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return result, nil
	}

	typeOfInput := value.Type()
	for i := range value.NumField() {
		field := value.Field(i)
		fieldType := typeOfInput.Field(i)

		key, omitEmpty := parseJSONTag(fieldType.Tag.Get("json"))
		if key == "-" {
			continue
		}

		if omitEmpty && isZeroValue(field) {
			continue
		}

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

func parseJSONTag(tag string) (string, bool) {
	if tag == constants.EmptyString {
		return constants.EmptyString, false
	}

	parts := strings.Split(tag, ",")
	fieldName := parts[constants.FirstIndex]

	omitEmpty := false
	for _, part := range parts[constants.SecondIndex:] {
		if strings.TrimSpace(part) == constants.OmitEmpty {
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
		return v.Int() == constants.FirstIndex
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == constants.FirstIndex
	case reflect.Float32, reflect.Float64:
		return v.Float() == constants.FirstIndex
	case reflect.String:
		return v.String() == constants.EmptyString
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == constants.FirstIndex
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Struct:
		return v.IsZero()
	default:
		return false
	}
}

func processFieldValue(field reflect.Value) (any, error) {
	if !field.CanInterface() {
		// Unexported or otherwise non-interfaceable field – skip it
		return nil, nil
	}

	switch field.Kind() {
	case reflect.Struct:
		return MapToJSONPayload(field.Interface())
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
	if length == constants.EmptySliceLength {
		return []any{}, nil
	}

	slice := make([]any, constants.EmptySliceLength, length)
	for i := range length {
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
	if field.Len() == constants.EmptySliceLength {
		return map[string]any{}, nil
	}

	result := make(map[string]any)
	iter := field.MapRange()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

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
