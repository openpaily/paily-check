package geo

import (
	"context"
	"strings"

	"github.com/openpaily/paily-check/internal/interfaces"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// GeoConfig carries parameters for a single Detect() call.
type GeoConfig struct {
	Mode            string             // "script" (default) | "mmdb" | "native_api"
	MmdbPath        string             // required when Mode == "mmdb"
	FallbackLegacy  bool               // allow native_api to fall back to legacy active geo
	FallbackInbound bool               // final MMDB fallback using node inbound address/domain
	IpScript        string             // optional custom IP-lookup JS; empty = DEFAULT_IP_SCRIPT
	GeoScript       string             // optional custom GeoIP JS;    empty = DEFAULT_GEOIP_SCRIPT
	Retry           int                // number of IP-lookup retries (≥ 1)
	NativeAPI       GeoNativeAPIConfig // native concurrent Geo API mode
}

// Detect resolves the exit country code for the given vendor proxy.
// Returns an ISO 3166-1 alpha-2 upper-case code (e.g. "HK", "US") or ""
// when detection fails.
func Detect(ctx context.Context, v interfaces.Vendor, cfg GeoConfig) string {
	if strings.EqualFold(cfg.Mode, "native_api") {
		if detected := detectNative(ctx, v, cfg); detected != "" {
			return detected
		}
		if !cfg.FallbackLegacy {
			if cfg.FallbackInbound {
				return fallbackNodeAddressMMDB(ctx, v, cfg.MmdbPath)
			}
			return ""
		}
	}
	if detected := detectLegacy(v, cfg); detected != "" {
		return detected
	}
	if !cfg.FallbackInbound {
		return ""
	}
	return fallbackNodeAddressMMDB(ctx, v, cfg.MmdbPath)
}

func detectNative(ctx context.Context, v interfaces.Vendor, cfg GeoConfig) string {
	if !strings.EqualFold(cfg.Mode, "native_api") {
		return ""
	}
	info := RunNativeGeoCheck(ctx, v, cfg.NativeAPI)
	if info != nil && info.CountryCode != "" {
		return strings.ToUpper(info.CountryCode)
	}
	return ""
}

func detectLegacy(v interfaces.Vendor, cfg GeoConfig) string {
	if v == nil || v.Status() == interfaces.VStatusNotReady {
		return ""
	}

	retry := cfg.Retry
	if retry < 1 {
		retry = 2
	}

	// ── 1. Resolve exit IP(s) ────────────────────────────────────────────────
	var ipstacks *interfaces.IPStacks
	for i := 0; i < retry && (ipstacks == nil || ipstacks.Count() == 0); i++ {
		ipstacks = ExecIpCheck(v, cfg.IpScript, interfaces.ROptionsTCP)
	}
	if ipstacks == nil || ipstacks.Count() == 0 {
		return ""
	}

	// ── 2. Resolve country for first available IP ────────────────────────────
	// Prefer IPv4; fall back to IPv6.
	candidates := append(ipstacks.IPv4, ipstacks.IPv6...)
	for _, ip := range candidates {
		if ip == "" {
			continue
		}
		geo := resolveGeo(v, ip, cfg)
		if geo != nil && geo.CountryCode != "" {
			return strings.ToUpper(geo.CountryCode)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func resolveGeo(v interfaces.Vendor, ip string, cfg GeoConfig) *interfaces.GeoInfo {
	if strings.EqualFold(cfg.Mode, "mmdb") && cfg.MmdbPath != "" {
		if g := RunMMDBCheck(cfg.MmdbPath, ip); g != nil && g.CountryCode != "" {
			return g
		}
		// fall through to script if mmdb returned nothing
	}

	network := interfaces.ROptionsTCP
	if strings.Contains(ip, ":") {
		network = interfaces.ROptionsTCP6
	}
	return ExecGeoCheck(v, cfg.GeoScript, ip, network)
}
