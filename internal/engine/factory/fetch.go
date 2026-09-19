package factory

import (
	"github.com/dop251/goja"

	"github.com/openpaily/paily-check/internal/engine/helpers"
	"github.com/openpaily/paily-check/internal/engine/request"
	"github.com/openpaily/paily-check/internal/interfaces"
)

// FetchFactory returns a JS fetch() implementation that routes requests
// through the given Vendor proxy.
func FetchFactory(vm *goja.Runtime, p interfaces.Vendor, network interfaces.RequestOptionsNetwork) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		url, _ := helpers.VMSafeStr(call.Argument(0))
		params, _ := helpers.VMSafeObj(vm, call.Argument(1))

		method := "GET"
		body := ""
		useHost := false
		noRedir := false
		retry := 0
		timeout := int64(3000)
		hdrs := map[string]string{}
		cookies := map[string]string{}

		if params != nil {
			if v, ok := helpers.VMSafeStr(params.Get("method")); ok {
				method = v
			}
			if v, ok := helpers.VMSafeStr(params.Get("body")); ok {
				body = v
			}
			if v, ok := helpers.VMSafeBool(params.Get("useHost")); ok {
				useHost = v
			}
			if v, ok := helpers.VMSafeBool(params.Get("noRedir")); ok {
				noRedir = v
			}
			if v, ok := helpers.VMSafeInt64(params.Get("retry")); ok {
				retry = int(v)
			}
			if v, ok := helpers.VMSafeInt64(params.Get("timeout")); ok {
				timeout = v
			}
			if vo, _ := helpers.VMSafeObj(vm, params.Get("headers")); vo != nil {
				for _, key := range vo.Keys() {
					if vv, ok := helpers.VMSafeStr(vo.Get(key)); ok {
						hdrs[key] = vv
					}
				}
			}
			if vo, _ := helpers.VMSafeObj(vm, params.Get("cookies")); vo != nil {
				for _, key := range vo.Keys() {
					if vv, ok := helpers.VMSafeStr(vo.Get(key)); ok {
						cookies[key] = vv
					}
				}
			}
		}

		vendor := p
		if useHost {
			vendor = nil
		}

		retBody, resp, redirs := request.WithRetry(vendor, retry, timeout, &interfaces.RequestOptions{
			Method:  method,
			URL:     url,
			Headers: hdrs,
			Cookies: cookies,
			Body:    []byte(body),
			NoRedir: noRedir,
			Network: network,
		})

		if resp == nil {
			return goja.Null()
		}

		retMap := map[string]interface{}{
			"status":     resp.Status,
			"statusCode": resp.StatusCode,
			"cookies":    resp.Cookies(),
			"headers":    resp.Header,
			"method":     method,
			"url":        url,
			"body":       string(retBody),
			// redirects: internal slice used for urlList below
			"redirected": len(redirs) > 0,
			"urlList":    redirs,
		}
		return vm.ToValue(retMap)
	}
}
