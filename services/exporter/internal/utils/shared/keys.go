package shared

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/telark/exporter/internal/constants"
)

var (
	jsonFieldsCache sync.Map
	unmarshalerType = reflect.TypeFor[json.Unmarshaler]()
)

// encoding/json matches keys case-insensitively while guards read exact keys,
// so a key that is not exactly a JSON name of T is refused before anything else sees it.
func CheckCanonicalKeys[T any](body map[string]any) error {
	return checkKeys(body, reflect.TypeFor[T](), constants.EmptyString)
}

func checkKeys(value any, t reflect.Type, path string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(unmarshalerType) {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		return checkStructKeys(value, t, path)
	case reflect.Slice, reflect.Array:
		list, isList := value.([]any)
		if !isList {
			return nil
		}
		for _, item := range list {
			if err := checkKeys(item, t.Elem(), path); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, isObject := value.(map[string]any)
		if !isObject {
			return nil
		}
		for key, item := range object {
			if err := checkKeys(item, t.Elem(), path+key+constants.BodyFieldPathSep); err != nil {
				return err
			}
		}
	default:
	}
	return nil
}

func checkStructKeys(value any, t reflect.Type, path string) error {
	object, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}
	fields := jsonFields(t)
	for key, item := range object {
		fieldType, known := fields[key]
		if !known {
			return fmt.Errorf(constants.ErrBodyFieldUnknown, path+key)
		}
		if err := checkKeys(item, fieldType, path+key+constants.BodyFieldPathSep); err != nil {
			return err
		}
	}
	return nil
}

func jsonFields(t reflect.Type) map[string]reflect.Type {
	if cached, found := jsonFieldsCache.Load(t); found {
		if fields, isFields := cached.(map[string]reflect.Type); isFields {
			return fields
		}
	}
	fields := map[string]reflect.Type{}
	collectJSONFields(t, fields)
	jsonFieldsCache.Store(t, fields)
	return fields
}

func collectJSONFields(t reflect.Type, fields map[string]reflect.Type) {
	for field := range t.Fields() {
		tag := field.Tag.Get(constants.JSONTagKey)
		if tag == constants.JSONTagSkip {
			continue
		}
		name, _, _ := strings.Cut(tag, constants.JSONTagOptionSep)
		if field.Anonymous && name == constants.EmptyString && embeddedStruct(field.Type) {
			collectJSONFields(derefType(field.Type), fields)
			continue
		}
		if !field.IsExported() {
			continue
		}
		if name == constants.EmptyString {
			name = field.Name
		}
		fields[name] = field.Type
	}
}

func embeddedStruct(t reflect.Type) bool {
	return derefType(t).Kind() == reflect.Struct
}

func derefType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}
