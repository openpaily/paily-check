package geo

import (
	"context"
	"net"
	"strings"

	"github.com/openpaily/paily-check/internal/interfaces"
)

func fallbackNodeAddressMMDB(ctx context.Context, v interfaces.Vendor, mmdbPath string) string {
	if v == nil || mmdbPath == "" {
		return ""
	}

	host := extractNodeHost(v.ProxyInfo().Address)
	if host == "" {
		return ""
	}

	if ip := net.ParseIP(host); ip != nil {
		if info := RunMMDBCheck(mmdbPath, ip.String()); info != nil && info.CountryCode != "" {
			return strings.ToUpper(info.CountryCode)
		}
		return ""
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		if info := RunMMDBCheck(mmdbPath, addr.IP.String()); info != nil && info.CountryCode != "" {
			return strings.ToUpper(info.CountryCode)
		}
	}
	return ""
}

func extractNodeHost(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	if strings.HasPrefix(address, "[") && strings.Contains(address, "]") {
		if host, _, err := net.SplitHostPort(address); err == nil {
			return strings.Trim(host, "[]")
		}
	}

	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}

	if ip := net.ParseIP(address); ip != nil {
		return ip.String()
	}

	return strings.Trim(address, "[]")
}
