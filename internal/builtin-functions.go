package internal

import "reflect"

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
