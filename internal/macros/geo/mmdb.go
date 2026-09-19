package geo

import (
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"

	"github.com/openpaily/paily-check/internal/interfaces"
)

// ---------------------------------------------------------------------------
// MMDB query types.
// ---------------------------------------------------------------------------

type mmdbRecord struct {
	ASN    int    `maxminddb:"autonomous_system_number"`
	ASNOrg string `maxminddb:"autonomous_system_organization"`

	City struct {
		Names struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"city"`

	Continent struct {
		Code  string `maxminddb:"code"`
		Names struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"continent"`

	Country struct {
		ISOCode string `maxminddb:"iso_code"`
		Names   struct {
			EN string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"country"`

	Location struct {
		TimeZone  string  `maxminddb:"time_zone"`
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// openMMDB caches the open database reader per path. A nil db means the path
// has not been opened yet.
var (
	mmdbCacheMu sync.Mutex
	mmdbCache   = map[string]*maxminddb.Reader{}
)

func openDB(path string) (*maxminddb.Reader, error) {
	if db, ok := mmdbCache[path]; ok {
		return db, nil
	}
	db, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	mmdbCache[path] = db
	return db, nil
}

// InvalidateMMDB closes and removes the cached MMDB reader for path so future
// lookups open the current file contents.
func InvalidateMMDB(path string) {
	mmdbCacheMu.Lock()
	defer mmdbCacheMu.Unlock()

	invalidateMMDBLocked(path)
}

func invalidateMMDBLocked(path string) {
	db, ok := mmdbCache[path]
	if ok {
		delete(mmdbCache, path)
	}
	if ok {
		_ = db.Close()
	}
}

func replaceCachedMMDBFile(src, dst string) error {
	mmdbCacheMu.Lock()
	defer mmdbCacheMu.Unlock()

	invalidateMMDBLocked(dst)
	return replaceFile(src, dst)
}

// RunMMDBCheck looks up rawIp in the MMDB file at mmdbPath.
// Returns nil when the IP cannot be parsed or the file cannot be opened.
func RunMMDBCheck(mmdbPath, rawIp string) *interfaces.GeoInfo {
	ip := net.ParseIP(rawIp)
	if ip == nil {
		return nil
	}

	mmdbCacheMu.Lock()
	defer mmdbCacheMu.Unlock()

	db, err := openDB(mmdbPath)
	if err != nil {
		return nil
	}

	var rec mmdbRecord
	if err := db.Lookup(ip, &rec); err != nil {
		return nil
	}

	return &interfaces.GeoInfo{
		ASN:           rec.ASN,
		ASNOrg:        rec.ASNOrg,
		Org:           rec.ASNOrg,
		ISP:           rec.ASNOrg,
		IP:            rawIp,
		Country:       rec.Country.Names.EN,
		CountryCode:   rec.Country.ISOCode,
		ContinentCode: rec.Continent.Code,
		TimeZone:      rec.Location.TimeZone,
		Lat:           float32(rec.Location.Latitude),
		Lon:           float32(rec.Location.Longitude),
	}
}
