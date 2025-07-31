package common

import "reflect"

func MapToJsonPayload(input any) map[string]any {
	result := make(map[string]any)
	value := reflect.ValueOf(input)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return result
	}

	typeOfInput := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := typeOfInput.Field(i)
		jsonTag := fieldType.Tag.Get("json")

		key := fieldType.Name
		if jsonTag != "" && jsonTag != "-" {
			key = jsonTag
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			result[key] = MapToJsonPayload(field.Interface())
		} else if field.Kind() == reflect.Slice {
			// Handle slices
			var slice []any
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if elem.Kind() == reflect.Struct {
					slice = append(slice, MapToJsonPayload(elem.Interface()))
				} else {
					slice = append(slice, elem.Interface())
				}
			}
			result[key] = slice
		} else {
			result[key] = field.Interface()
		}
	}
	return result
}
