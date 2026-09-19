package factory

import (
	"github.com/dop251/goja"

	"github.com/openpaily/paily-check/internal/engine/helpers"
	"github.com/openpaily/paily-check/internal/engine/request"
	"github.com/openpaily/paily-check/internal/interfaces"
)

// NetCatFactory returns a JS netcat() implementation that sends raw data
// through the vendor proxy.
func NetCatFactory(vm *goja.Runtime, p interfaces.Vendor, network interfaces.RequestOptionsNetwork) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		addr, _ := helpers.VMSafeStr(call.Argument(0))
		data, _ := helpers.VMSafeStr(call.Argument(1))
		params, _ := helpers.VMSafeObj(vm, call.Argument(2))

		retry := 0
		useHost := false
		timeout := int64(3000)
		readLine := false

		if params != nil {
			if v, ok := helpers.VMSafeBool(params.Get("useHost")); ok {
				useHost = v
			}
			if v, ok := helpers.VMSafeInt64(params.Get("timeout")); ok {
				timeout = v
			}
			if v, ok := helpers.VMSafeInt64(params.Get("retry")); ok {
				retry = int(v)
			}
			if v, ok := helpers.VMSafeBool(params.Get("readLine")); ok {
				readLine = v
			}
		}

		vendor := p
		if useHost {
			vendor = nil
		}

		returns, err := request.NetCatWithRetry(vendor, retry, timeout, addr, []byte(data), readLine, network)

		retMap := map[string]string{
			"error": "",
			"data":  string(returns),
		}
		if err != nil {
			retMap["error"] = err.Error()
		}
		return vm.ToValue(retMap)
	}
}
