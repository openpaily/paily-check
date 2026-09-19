package interfaces

import (
	"context"
	"net"
)

type VendorType string

const (
	VendorLocal   VendorType = "Local"
	VendorClash   VendorType = "Clash"
	VendorInvalid VendorType = "Invalid"
)

type VendorStatus uint

const (
	VStatusOperational VendorStatus = iota
	VStatusNotReady
)

// Vendor is the proxy dial interface used by all macros.
// Build accepts the raw Clash proxy object (map[string]interface{}) as parsed
// from the node's raw field.
type Vendor interface {
	// Type returns the vendor implementation type.
	Type() VendorType

	// Status returns whether the vendor is ready to dial.
	Status() VendorStatus

	// Build constructs a new Vendor from a raw Clash proxy map.
	// raw is the map[string]interface{} from the node's raw field.
	Build(raw map[string]interface{}) Vendor

	// DialTCP dials a TCP connection through the proxy to url.
	DialTCP(ctx context.Context, url string, network RequestOptionsNetwork) (net.Conn, error)

	// DialUDP opens a UDP packet connection through the proxy to url.
	DialUDP(ctx context.Context, url string) (net.PacketConn, error)

	// ProxyInfo returns human-readable proxy metadata.
	ProxyInfo() ProxyInfo

	// ProxyProfile returns a JSON representation of the raw proxy map.
	ProxyProfile() string
}
