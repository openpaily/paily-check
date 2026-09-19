// Package geo implements geographic IP detection for a single proxy node.
package geo

import (
	"net"
	"strings"

	"github.com/openpaily/paily-check/internal/engine"
	"github.com/openpaily/paily-check/internal/engine/helpers"
	"github.com/openpaily/paily-check/internal/interfaces"
)

// ExecIpCheck runs the IP-resolution JS snippet through the vendor proxy and
// returns a populated IPStacks containing the exit IPv4 and IPv6 addresses.
func ExecIpCheck(p interfaces.Vendor, script string, network interfaces.RequestOptionsNetwork) *interfaces.IPStacks {
	ipstacks := (&interfaces.IPStacks{}).Init()

	vm := engine.VMNewWithVendor(p, network)
	vm.RunString(engine.PREDEFINED_SCRIPT + engine.DEFAULT_IP_SCRIPT + script) //nolint:errcheck

	caller := "ip_resolve_default"
	if engine.HasFunction(vm, "ip_resolve") {
		caller = "ip_resolve"
	}

	ret, err := engine.ExecTaskCallback(vm, caller)
	if engine.ThrowExecTaskErr("IPResolve", err) {
		return ipstacks
	}

	ipQuery := []string{}
	helpers.VMSafeMarshal(&ipQuery, ret, vm) //nolint:errcheck
	for _, ip := range ipQuery {
		if net.ParseIP(ip) == nil {
			continue
		}
		if strings.Contains(ip, ":") {
			ipstacks.IPv6 = append(ipstacks.IPv6, ip)
		} else {
			ipstacks.IPv4 = append(ipstacks.IPv4, ip)
		}
	}
	return ipstacks
}

// ExecGeoCheck resolves geographic metadata for a single IP address using
// a JS handler function; falls back to DEFAULT_GEOIP_SCRIPT when script == "".
func ExecGeoCheck(p interfaces.Vendor, script string, ip string, network interfaces.RequestOptionsNetwork) *interfaces.GeoInfo {
	vm := engine.VMNewWithVendor(p, network)
	if script == "" {
		script = engine.DEFAULT_GEOIP_SCRIPT
	}
	vm.RunString(engine.PREDEFINED_SCRIPT + script) //nolint:errcheck

	ret, err := engine.ExecTaskCallback(vm, "handler", ip)
	if engine.ThrowExecTaskErr("GeoCheck", err) {
		return &interfaces.GeoInfo{}
	}

	geoInfo := &interfaces.GeoInfo{}
	if err := helpers.VMSafeMarshal(geoInfo, ret, vm); err == nil {
		return geoInfo
	}
	return &interfaces.GeoInfo{}
}
