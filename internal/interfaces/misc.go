package interfaces

// RequestOptionsNetwork specifies the TCP stack to use when dialling.
type RequestOptionsNetwork string

const (
	ROptionsTCP  RequestOptionsNetwork = "tcp"
	ROptionsTCP6 RequestOptionsNetwork = "tcp6"
)

// String returns the string value, defaulting to "tcp".
func (ron *RequestOptionsNetwork) String() string {
	if ron == nil {
		return "tcp"
	}
	switch *ron {
	case ROptionsTCP:
		return "tcp"
	case ROptionsTCP6:
		return "tcp6"
	}
	return "tcp"
}

// RequestOptions carries parameters for an outbound HTTP request made through
// a Vendor (used by the JS engine's fetch() factory).
type RequestOptions struct {
	Method  string
	URL     string
	DialURL string
	Headers map[string]string
	Cookies map[string]string
	Body    []byte
	NoRedir bool
	Network RequestOptionsNetwork
}
