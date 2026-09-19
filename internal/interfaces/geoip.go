package interfaces

import (
	"sort"
	"strings"
)

// IPStacks holds the IPv4 and IPv6 addresses resolved for a proxy's exit IP.
type IPStacks struct {
	IPv4 []string
	IPv6 []string
}

// Init initialises empty slices and returns the receiver.
func (ips *IPStacks) Init() *IPStacks {
	if ips == nil {
		ips = &IPStacks{}
	}
	ips.IPv4 = []string{}
	ips.IPv6 = []string{}
	return ips
}

// Count returns the total number of IP addresses.
func (ips *IPStacks) Count() int {
	if ips == nil {
		return 0
	}
	return len(ips.IPv4) + len(ips.IPv6)
}

// GeoInfo holds geographic and network metadata for a single IP address.
type GeoInfo struct {
	Org           string  `json:"organization"`
	Lon           float32 `json:"longitude"`
	Lat           float32 `json:"latitude"`
	TimeZone      string  `json:"timezone"`
	ISP           string  `json:"isp"`
	ASN           int     `json:"asn"`
	ASNOrg        string  `json:"asn_organization"`
	Country       string  `json:"country"`
	IP            string  `json:"ip"`
	ContinentCode string  `json:"continent_code"`
	CountryCode   string  `json:"country_code"`
	CountryNameZH string  `json:"country_name_zh"`
	StackType     string  `json:"stackType"`
}

// IsV6 returns true when the IP is an IPv6 address.
func (gi *GeoInfo) IsV6() bool {
	return gi != nil && gi.IP != "" && strings.Contains(gi.IP, ":")
}

// MultiStacks groups GeoInfo results for both IPv4 and IPv6 stacks.
type MultiStacks struct {
	Domain    string   // node name / domain identifier
	MainStack *GeoInfo // deprecated; use IPv4Stack/IPv6Stack
	IPv4Stack []*GeoInfo
	IPv6Stack []*GeoInfo
}

// Count returns the total number of geo entries.
func (tms *MultiStacks) Count() int {
	if tms == nil {
		return 0
	}
	v4, v6 := tms.V46StackCount()
	return v4 + v6
}

// V46StackCount returns separate counts for IPv4 and IPv6 stacks.
func (tms *MultiStacks) V46StackCount() (int, int) {
	if tms == nil {
		return 0, 0
	}
	return len(tms.IPv4Stack), len(tms.IPv6Stack)
}

// Repr returns a sorted comma-separated list of all IPs.
func (tms *MultiStacks) Repr() string {
	if tms == nil || tms.Count() == 0 {
		return ""
	}
	repr := []string{}
	for _, v4 := range tms.IPv4Stack {
		repr = append(repr, v4.IP)
	}
	for _, v6 := range tms.IPv6Stack {
		repr = append(repr, v6.IP)
	}
	sort.Strings(repr)
	return strings.Join(repr, ",")
}

// First returns the first non-empty GeoInfo entry.
// tag: "v4" prefers IPv4, "v6" prefers IPv6, anything else tries IPv4 first.
func (tms *MultiStacks) First(tag string) *GeoInfo {
	if tms == nil || tms.Count() == 0 {
		return nil
	}
	if tag == "v6" {
		for _, v6 := range tms.IPv6Stack {
			if v6.IP != "" {
				return v6
			}
		}
		for _, v4 := range tms.IPv4Stack {
			if v4.IP != "" {
				return v4
			}
		}
		return nil
	}
	for _, v4 := range tms.IPv4Stack {
		if v4.IP != "" {
			return v4
		}
	}
	for _, v6 := range tms.IPv6Stack {
		if v6.IP != "" {
			return v6
		}
	}
	return nil
}
