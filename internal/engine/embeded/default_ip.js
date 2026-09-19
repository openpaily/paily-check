function get_ip_by_ipsb() {
    const urls = ["https://api-ipv4.ip.sb/ip", "https://api-ipv6.ip.sb/ip"];
    const ipret = [];
    urls.forEach((url) => {
        const content = (get(fetch(url, {
            headers: {
                'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Safari/537.36',
            },
            retry: 1,
            timeout: 3000,
        }), "body") || "").trim();

        if (content !== "") {
            ipret.push(content);
        }
    });
    return ipret;
}

const ip_resolve_default = get_ip_by_ipsb;