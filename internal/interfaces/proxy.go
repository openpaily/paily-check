package interfaces

// ProxyType enumerates supported Clash proxy protocols.
type ProxyType string

const (
	Shadowsocks  ProxyType = "Shadowsocks"
	ShadowsocksR ProxyType = "ShadowsocksR"
	Snell        ProxyType = "Snell"
	Socks5       ProxyType = "Socks5"
	Http         ProxyType = "Http"
	Vmess        ProxyType = "Vmess"
	Trojan       ProxyType = "Trojan"
	Vless        ProxyType = "Vless"
	Hysteria     ProxyType = "Hysteria"
	Hysteria2    ProxyType = "Hysteria2"
	Tuic         ProxyType = "Tuic"
	Wireguard    ProxyType = "Wireguard"
	AnyTLS       ProxyType = "AnyTLS"
	Mieru        ProxyType = "Mieru"
	TrustTunnel  ProxyType = "Trusttunnel"
	Masque       ProxyType = "Masque"

	ProxyInvalid ProxyType = "Invalid"
)

var AllProxyTypes = []ProxyType{
	Shadowsocks, ShadowsocksR, Snell, Socks5, Http, Vmess, Trojan,
	Vless, Hysteria, Hysteria2, Tuic, Wireguard, AnyTLS, Mieru, TrustTunnel, Masque,
}

// Valid returns true if proxyType is a known protocol.
func Valid(proxyType ProxyType) bool {
	for _, t := range AllProxyTypes {
		if t == proxyType {
			return true
		}
	}
	return false
}

// Parse normalises a raw type string into a ProxyType.
func Parse(proxyType string) ProxyType {
	switch proxyType {
	case "ss":
		proxyType = string(Shadowsocks)
	case "ssr":
		proxyType = string(ShadowsocksR)
	}
	pt := ProxyType(proxyType)
	if Valid(pt) {
		return pt
	}
	return ProxyInvalid
}

// ProxyInfo carries human-readable metadata about a proxy.
type ProxyInfo struct {
	Name    string
	Address string
	Type    ProxyType
}

// Map returns a string map representation used by the JS VM.
func (pi *ProxyInfo) Map() map[string]string {
	return map[string]string{
		"Name":    pi.Name,
		"Address": pi.Address,
		"Type":    string(pi.Type),
	}
}
