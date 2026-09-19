const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Safari/537.36"
function handler_ipleak(ip) {
    const isv6 = ip.includes(":")
    let geoip_api = `https://ipv4.ipleak.net/json/${ip}`
    if (isv6) {
        geoip_api = `https://ipv4.ipleak.net/json/${ip}`
    }
    const content = fetch(geoip_api, {
        headers: {
            'User-Agent': UA,
        },
        retry: 1,
        timeout: 3000,
    });
    const ret = safeParse(get(content, "body"));
    return {
        "ip": get(ret, "query", ""),
        "isp": get(ret, "isp_name", ""),
        "organization": get(ret, "isp_name", ""),
        "latitude": get(ret, "latitude", 0),
        "longitude": get(ret, "longitude", 0),
        "asn": parseInt(get(ret, "as_number", 0), 10) || 0,
        "asn_organization": get(ret, "isp_name", ""),
        "timezone": get(ret, "time_zone", ""),
        "region": get(ret, "region_name", ""),
        "city": get(ret, "city", ""),
        "country": get(ret, "city_name", ""),
        "country_code": get(ret, "country_code", ""),
    }
}

function handler(ip) {
    let result = {};
    result = handler_ipleak(ip)
    if (result && result.ip) {
        return result;
    }
    return result;
}
