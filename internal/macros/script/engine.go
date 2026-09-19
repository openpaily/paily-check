// Package script implements JS streaming-script execution returning bool.
package script

import (
	"runtime"
	"time"

	"github.com/dop251/goja"

	"github.com/openpaily/paily-check/internal/engine"
	"github.com/openpaily/paily-check/internal/engine/helpers"
	"github.com/openpaily/paily-check/internal/interfaces"
)

// ExecScript runs a single JS script against the vendor proxy and returns
// a ScriptResult. The JS handler must return one of:
//   - true / false         → taken directly
//   - {unlocked: bool}    → .unlocked field extracted
//   - anything else / error / timeout → false
func ExecScript(p interfaces.Vendor, script *interfaces.Script) interfaces.ScriptResult {
	s := interfaces.ScriptResult{}
	if script == nil {
		return s
	}

	vm := engine.VMNewWithVendor(p, interfaces.ROptionsTCP)

	startTime := time.Now()
	ret, err := engine.RunWithTimeout(vm, time.Duration(script.TimeoutMillis)*time.Millisecond, func() (goja.Value, error) {
		vm.RunString(engine.PREDEFINED_SCRIPT + script.Content) //nolint:errcheck
		return engine.ExecTaskCallback(vm, "handler")
	})

	s.TimeElapsed = time.Now().UnixMilli() - startTime.UnixMilli()

	if !engine.ThrowExecTaskErr("MediaTest", err) {
		// JS returned a direct bool
		if b, ok := helpers.VMSafeBool(ret); ok {
			s.Unlocked = b
		} else if ro, _ := helpers.VMSafeObj(vm, ret); ro != nil {
			// JS returned {unlocked: bool, ...}
			if b, ok := helpers.VMSafeBool(ro.Get("unlocked")); ok {
				s.Unlocked = b
			}
		}
		// any other return value → s.Unlocked remains false
	}

	vm = nil
	runtime.GC()

	return s
}
