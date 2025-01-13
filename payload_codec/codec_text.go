package payloadcodec

import (
	"fmt"
	"reflect"
)

type Text struct{}

func (c Text) Marshal(v any) ([]byte, error) {
	switch v := v.(type) {
	case string:
		return []byte(v), nil
	case *string:
		if v == nil {
			return nil, fmt.Errorf("nil string pointer")
		}
		return []byte(*v), nil
	case []byte:
		return v, nil
	case *[]byte:
		if v == nil {
			return nil, fmt.Errorf("nil byte slice pointer")
		}
		return *v, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

func (c Text) Unmarshal(data []byte, v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Pointer && !val.Elem().CanSet() {
		return fmt.Errorf("cannot set value of unaddressable pointer")
	}
	switch val.Kind() {
	case reflect.Pointer:
		elem := val.Elem()
		if elem.Kind() == reflect.String {
			elem.SetString(string(data))
		} else if elem.Kind() == reflect.Slice && elem.Type().Elem().Kind() == reflect.Uint8 {
			elem.SetBytes(data)
		} else {
			return fmt.Errorf("unsupported pointer type %T", v)
		}
	default:
		return fmt.Errorf("unsupported type %T", v)
	}
	return nil
}

func (c Text) IsNull(data []byte) bool {
	return len(data) == 0
}

func (c Text) Name() string {
	return "text"
}
