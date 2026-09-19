package helpers

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/dop251/goja"
)

// VMCheck returns true when v is a non-null, non-undefined JS value.
func VMCheck(v goja.Value) bool {
	return v != nil && !goja.IsNull(v) && !goja.IsUndefined(v)
}

// VMSafeStr extracts a string from a JS value.
func VMSafeStr(v goja.Value) (string, bool) {
	if VMCheck(v) && v.ExportType() != nil && v.ExportType().Kind() == reflect.String {
		return v.Export().(string), true
	}
	return "", false
}

// VMSafeBool extracts a bool from a JS value.
func VMSafeBool(v goja.Value) (bool, bool) {
	if VMCheck(v) && v.ExportType() != nil && v.ExportType().Kind() == reflect.Bool {
		return v.Export().(bool), true
	}
	return false, false
}

// VMSafeInt64 extracts an int64 from a JS value.
func VMSafeInt64(v goja.Value) (int64, bool) {
	if VMCheck(v) && v.ExportType() != nil && v.ExportType().Kind() == reflect.Int64 {
		return v.Export().(int64), true
	}
	return 0, false
}

// VMSafeObj extracts an Object pointer from a JS value.
func VMSafeObj(vm *goja.Runtime, v goja.Value) (*goja.Object, bool) {
	if VMCheck(v) && v.ExportType() != nil && v.ExportType().Kind() == reflect.Map {
		vo := v.ToObject(vm)
		if vo != nil {
			return vo, true
		}
	}
	return nil, false
}

// VMSafeMarshal serialises the JS value to JSON then unmarshals it into target.
// Requires PREDEFINED_SCRIPT to have already been loaded (provides __json_stringify).
func VMSafeMarshal(target interface{}, obj goja.Value, vm *goja.Runtime) error {
	if fn, ok := goja.AssertFunction(vm.Get("__json_stringify")); ok {
		ret, _ := fn(goja.Undefined(), obj)
		if v, ok := VMSafeStr(ret); ok {
			if v == "" {
				return fmt.Errorf("cannot marshal an empty string")
			}
			return json.Unmarshal([]byte(v), target)
		}
		return fmt.Errorf("cannot read from stringify function")
	}
	return fmt.Errorf("cannot find stringify function")
}
