package vendor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/component/resolver"
	"github.com/metacubex/mihomo/constant"

	"github.com/openpaily/paily-check/internal/interfaces"
)

// ---------------------------------------------------------------------------
// ClashVendor — Mihomo-backed implementation of interfaces.Vendor
// ---------------------------------------------------------------------------

// ClashVendor dials outbound connections through a Clash/Mihomo proxy.
// Build must be called before any Dial method is used.
type ClashVendor struct {
	proxy     constant.Proxy
	raw       map[string]interface{}
	name      string
	addr      string
	proxyType string
}

// New returns a zero-value ClashVendor ready to be configured via Build.
func New() interfaces.Vendor {
	return &ClashVendor{}
}

func (v *ClashVendor) Type() interfaces.VendorType {
	return interfaces.VendorClash
}

func (v *ClashVendor) Status() interfaces.VendorStatus {
	if v == nil || v.proxy == nil {
		return interfaces.VStatusNotReady
	}
	return interfaces.VStatusOperational
}

// Build constructs a new ClashVendor from the raw Clash proxy map.
// raw is the map[string]interface{} stored in client.Node.Raw.
// Numbers in the map may be float64 (from JSON); mihomo handles the coercion.
func (v *ClashVendor) Build(raw map[string]interface{}) interfaces.Vendor {
	n := &ClashVendor{raw: raw}

	// extract human-readable metadata before parsing
	if val, ok := raw["name"]; ok {
		n.name, _ = val.(string)
	}
	if val, ok := raw["server"]; ok {
		n.addr, _ = val.(string)
	}
	if val, ok := raw["type"]; ok {
		n.proxyType, _ = val.(string)
	}

	proxy, err := adapter.ParseProxy(raw)
	if err != nil {
		// Return a vendor in NotReady state so callers skip this node.
		return n
	}
	// Only accept protocols we explicitly support.
	if interfaces.Parse(proxy.Type().String()) == interfaces.ProxyInvalid {
		return n
	}
	n.proxy = proxy
	return n
}

// DialTCP dials a TCP connection through the proxy to the given URL.
// url must be parseable (scheme://host[:port]).
func (v *ClashVendor) DialTCP(ctx context.Context, rawURL string, network interfaces.RequestOptionsNetwork) (net.Conn, error) {
	if v == nil || v.proxy == nil {
		return nil, fmt.Errorf("vendor: proxy not built")
	}

	meta, err := urlToMetadata(rawURL, constant.TCP)
	if err != nil {
		return nil, fmt.Errorf("vendor: build tcp metadata: %w", err)
	}

	// Ensure IPv6 resolution is not forcibly disabled.
	if resolver.DisableIPv6 {
		resolver.DisableIPv6 = false
	}

	return v.proxy.DialContext(ctx, &meta)
}

// DialUDP dials a UDP packet connection through the proxy.
func (v *ClashVendor) DialUDP(ctx context.Context, rawURL string) (net.PacketConn, error) {
	if v == nil || v.proxy == nil {
		return nil, fmt.Errorf("vendor: proxy not built")
	}

	meta, err := urlToMetadata(rawURL, constant.UDP)
	if err != nil {
		return nil, fmt.Errorf("vendor: build udp metadata: %w", err)
	}

	return v.proxy.ListenPacketContext(ctx, &meta)
}

// ProxyInfo returns human-readable metadata about the underlying proxy.
func (v *ClashVendor) ProxyInfo() interfaces.ProxyInfo {
	if v == nil {
		return interfaces.ProxyInfo{}
	}
	if v.proxy != nil {
		return interfaces.ProxyInfo{
			Name:    v.proxy.Name(),
			Address: v.proxy.Addr(),
			Type:    interfaces.Parse(capitalize(v.proxy.Type().String())),
		}
	}
	// proxy failed to parse — return metadata extracted from the raw map
	return interfaces.ProxyInfo{
		Name:    v.name,
		Address: v.addr,
		Type:    interfaces.Parse(capitalize(v.proxyType)),
	}
}

// ProxyProfile returns a JSON representation of the raw proxy map.
// This is used by the JS engine factory and geo scripts.
func (v *ClashVendor) ProxyProfile() string {
	if v == nil || v.raw == nil {
		return "{}"
	}
	b, err := json.Marshal(v.raw)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// urlToMetadata parses a URL into a mihomo Metadata struct used for dialling.
func urlToMetadata(rawURL string, network constant.NetWork) (constant.Metadata, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return constant.Metadata{}, err
	}

	portStr := u.Port()
	if portStr == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			portStr = "443"
		case "http":
			portStr = "80"
		default:
			return constant.Metadata{}, fmt.Errorf("vendor: unsupported scheme %q in %s", u.Scheme, rawURL)
		}
	}

	portU64, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return constant.Metadata{}, fmt.Errorf("vendor: invalid port %q: %w", portStr, err)
	}

	return constant.Metadata{
		NetWork: network,
		Host:    u.Hostname(),
		DstIP:   netip.Addr{},
		DstPort: uint16(portU64),
	}, nil
}

// capitalize upper-cases the first rune and lower-cases the rest.
// Used to normalise proxy type strings from mihomo (e.g. "VMESS" → "Vmess").
func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
