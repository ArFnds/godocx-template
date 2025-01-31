package internal

import (
	"reflect"
	"strings"
)

func length(args ...any) VarValue {
	reflectValue := reflect.ValueOf(args[0])
	if reflectValue.Kind() != reflect.Slice &&
		reflectValue.Kind() != reflect.Map &&
		reflectValue.Kind() != reflect.String &&
		reflectValue.Kind() != reflect.Array {
		return -1
	}
	return reflectValue.Len()
}

func join(args ...any) VarValue {
	if len(args) != 2 {
		return ""
	}
	separator, okSep := args[1].(string)
	if !okSep {
		return ""
	}
	if arr, ok := args[0].([]any); ok {
		arrStr := make([]string, len(arr))
		for i := 0; i < len(arr); i++ {
			if _, ok := arr[i].(string); !ok {
				return ""
			}
			arrStr[i] = arr[i].(string)
		}
		return strings.Join(arrStr, separator)
	}
	return ""
}
