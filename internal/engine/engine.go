package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"

	"github.com/openpaily/paily-check/internal/engine/factory"
	"github.com/openpaily/paily-check/internal/interfaces"
)

// VMNew creates a bare JS runtime with console and print/debug globals.
func VMNew() *goja.Runtime {
	rt := goja.New()
	new(require.Registry).Enable(rt)
	console.Enable(rt)

	rt.Set("print", factory.PrintFactory(rt, "Script Print |"))
	rt.Set("debug", factory.PrintFactory(rt, "Script Debug |"))

	rt.SetMaxCallStackSize(1024)
	return rt
}

// VMNewWithVendor creates a JS runtime wired to a vendor for fetch/netcat.
func VMNewWithVendor(p interfaces.Vendor, network interfaces.RequestOptionsNetwork) *goja.Runtime {
	vm := VMNew()

	if p != nil {
		pi := p.ProxyInfo()
		vm.Set("proxy", vm.ToValue(pi.Map()))
	} else {
		vm.Set("proxy", nil)
	}

	vm.Set("fetch", factory.FetchFactory(vm, p, network))
	vm.Set("netcat", factory.NetCatFactory(vm, p, network))

	return vm
}

// IsNotExtractError reports whether err is the "cannot extract function" sentinel.
func IsNotExtractError(err error) bool {
	return err != nil && err.Error() == "cannot extract function from vm"
}

// ThrowExecTaskErr logs the error and returns whether an error occurred.
func ThrowExecTaskErr(scenario string, err error) bool {
	if err != nil {
		if !IsNotExtractError(err) {
			// non-fatal: log and carry on
			_ = scenario // available for structured logging if needed
		}
		return true
	}
	return false
}

// HasFunction returns true when vm exposes a callable with the given name.
func HasFunction(vm *goja.Runtime, caller string) bool {
	_, ok := goja.AssertFunction(vm.Get(caller))
	return ok
}

// ExecTaskCallback calls a JS function by name with the supplied Go args.
func ExecTaskCallback(vm *goja.Runtime, caller string, args ...interface{}) (ret goja.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ExecTaskCallback panic: %v", r)
		}
	}()

	if vm == nil {
		return goja.Undefined(), fmt.Errorf("vm is not initialized")
	}

	fn, ok := goja.AssertFunction(vm.Get(caller))
	if !ok {
		return goja.Undefined(), fmt.Errorf("cannot extract function from vm")
	}

	values := make([]goja.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, vm.ToValue(arg))
	}

	return fn(goja.Undefined(), values...)
}

// RunWithTimeout runs fn in the current goroutine and interrupts the vm if
// timeout elapses before fn returns. timeout ≤ 0 means no timeout.
func RunWithTimeout(vm *goja.Runtime, timeout time.Duration, fn func() (goja.Value, error)) (ret goja.Value, err error) {
	vmLock := sync.Mutex{}
	finished := false

	if timeout > 0 {
		// clamp to [1s, 60s]
		if timeout < time.Second {
			timeout = time.Second
		}
		if timeout > time.Minute {
			timeout = time.Minute
		}

		time.AfterFunc(timeout, func() {
			vmLock.Lock()
			defer vmLock.Unlock()
			if !finished {
				finished = true
				vm.Interrupt("script executing too long")
			}
		})
	}

	ret, err = fn()

	vmLock.Lock()
	finished = true
	vmLock.Unlock()

	return
}
