# 脚本开发

Paily 测活后端提供了可扩展的流媒体解锁检测能力。您可以自己编写自定义的解锁脚本来进行测试。

## 介绍 
Paily 测活后端针对解锁检测提供了两种接口：JS 脚本和 Native Handler。

JS 脚本用于灵活检测，您无需编译项目，只需要把检测脚本放到脚本目录即可，相关设计参考了 [miaospeed](https://github.com/miaokobot/miaospeed) 。如果您对相关项目的脚本开发熟悉，您可能很快上手。

Native Handler 则将此类能力集成在源码中编译为二进制，在实际部署中，如果您发现 JS 脚本在节点数量过多时出现了性能瓶颈，您可以切换为编写 Native Handler 并且将相关逻辑编译进可执行文件。

在实际测试中，Native Handler 性能比 JS 脚本快大约 50% - 60% 。

## 开发 JS 脚本

将自定义检测 JS 脚本放入`deep.script.dir` 指定的目录，文件名去掉 `.js` 后就是上报结果中的键名。
例如 `scripts/example.js` 的结果键为 `example`。

脚本使用 Goja 运行，必须提供同步的全局 `handler()` 函数。返回 `true` 表示
检测通过，返回 `false` 表示不通过；也可返回 `{ unlocked: true }` 或
`{ unlocked: false }`。异常、超时或其他返回值均按 `false` 处理。

```js
function handler() {
  return true;
}
```

在配置中使用 `deep.script.engine: goja` 启用 JS 脚本。`deep.script.scripts`
可限制加载的文件名，`deep.script.regions` 可限制哪些出口地区执行脚本。

### 可用全局接口

#### `fetch(url, options)`

通过当前被测节点发送 HTTP 请求。默认经节点代理出站；请求失败时返回 `null`。

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `url` | `string` | 必填 | 完整 HTTP 或 HTTPS URL |
| `options.method` | `string` | `"GET"` | HTTP 方法 |
| `options.body` | `string` | `""` | 请求体 |
| `options.headers` | `object` | `{}` | 请求头，值必须为字符串 |
| `options.cookies` | `object` | `{}` | Cookie 名称到值的映射 |
| `options.timeout` | `number` | `3000` | 单次请求超时，单位毫秒 |
| `options.retry` | `number` | `0` | 请求次数，实际范围为 1 到 10；`0` 等同于 1 次 |
| `options.noRedir` | `boolean` | `false` | 为 `true` 时不跟随重定向 |
| `options.useHost` | `boolean` | `false` | 为 `true` 时由检查器主机直连，绕过被测节点代理 |

成功时返回：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `status` | `string` | 状态行，例如 `"200 OK"` |
| `statusCode` | `number` | HTTP 状态码 |
| `body` | `string` | 完整响应体 |
| `headers` | `object` | 响应头；一个头可对应多个字符串值 |
| `cookies` | `array` | 响应中的 Cookie 列表 |
| `method` | `string` | 实际请求方法 |
| `url` | `string` | 传入的请求 URL |
| `redirected` | `boolean` | 是否发生过重定向 |
| `urlList` | `array` | 已记录的重定向 `Location` 值 |


#### `netcat(address, data, options)`

通过当前被测节点建立原始 TCP 连接、写入数据并读取响应。`address` 必须是带协议的
URL，例如 `https://example.com:443` 或 `http://example.com:80`；它用于确定目标主机和端口。

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `address` | `string` | 必填 | 目标 URL，协议仅支持 `http`、`https` |
| `data` | `string` | 必填 | 写入连接的原始文本 |
| `options.timeout` | `number` | `3000` | 单次连接超时，单位毫秒 |
| `options.retry` | `number` | `0` | 请求次数，实际范围为 1 到 10 |
| `options.readLine` | `boolean` | `false` | 为 `true` 时最多读取一次、至多 4096 字节 |
| `options.useHost` | `boolean` | `false` | 为 `true` 时由检查器主机直连 |

始终返回对象 `{ data, error }`。`error` 为空字符串表示成功；`data` 是原始响应文本。

#### 其他全局对象与函数

| 名称 | 类型 | 说明 |
| --- | --- | --- |
| `proxy` | `object` | 当前节点元信息，包含 `Name`、`Address`、`Type`；节点不可用时可能为空 |
| `print(...values)` | `function` | 写入检查器日志，并返回格式化后的字符串 |
| `debug(...values)` | `function` | 写入检查器日志，并返回格式化后的字符串 |
| `get(value, path, fallback)` | `function` | 通过点分路径读取对象字段，缺失时返回 `fallback`，默认值为 `null` |
| `safeParse(text)` | `function` | JSON 解析；失败时返回 `{}` |
| `safeStringify(value)` | `function` | JSON 序列化；失败时返回空字符串 |
| `println(...values)` | `function` | `print` 的别名 |

### 完整 JS 示例

下面的 `scripts/example.js` 检查一个 HTTP 服务是否可通过节点访问。它展示了节点元信息、
请求头、超时、重试、响应解析、重定向控制和日志。

```js
function handler() {
  print("checking through proxy:", safeStringify(proxy));

  const response = fetch("https://example.com/health", {
    method: "GET",
    headers: {
      "Accept": "application/json",
      "User-Agent": "paily-check-script/1.0",
    },
    timeout: 5000,
    retry: 2,
    noRedir: true,
  });

  if (response === null) {
    debug("request failed");
    return false;
  }

  if (response.statusCode !== 200) {
    debug("unexpected status:", response.status);
    return false;
  }

  const payload = safeParse(response.body);
  return get(payload, "status") === "ok";
}
```
如果您拥有 Miaospeed 的流媒体检测脚本，只需将返回值更改为对应格式即可。

## 开发出口地区检测脚本

当 `deep.geo.mode` 为 `script` 时，程序先执行出口 IP 脚本，再对每个得到的 IP 执行 GeoIP 脚本。您可以通过 `deep.geo.ip_script` 和 `deep.geo.geo_script` 或者对应的环境变量分别配置两段 JS 脚本。

`ip_script` 必须定义全局函数 `ip_resolve()`，并返回字符串数组。数组中的每项必须是可解析的 IPv4 或 IPv6 地址；无效值会被忽略。未定义 `ip_resolve()` 时使用内置 IP查询逻辑。

`geo_script` 必须定义全局函数 `handler(ip)`，参数 `ip` 是待查询的出口 IP。函数应返回对象，至少包含 `ip` 和两位大写 ISO 3166-1 国家代码 `country_code`。最终地区判定只使用 `country_code`；其余字段会解析为网络元数据，可按需返回。

```yaml
deep:
  geo:
    mode: script
    ip_script: |-
      function ip_resolve() {
        return ["203.0.113.1"];
      }
    geo_script: |-
      function handler(ip) {
        return { ip: ip, country_code: "US" };
      }
```

| 字段 | 类型 | 用途 |
| --- | --- | --- |
| `ip` | `string` | 查询到的 IP 地址 |
| `country_code` | `string` | 两位国家或地区代码，例如 `US`、`JP`、`HK` |
| `country` | `string` | 国家或地区名称 |
| `organization` | `string` | 网络组织名称 |
| `isp` | `string` | ISP 名称 |
| `asn` | `number` | ASN 编号 |
| `asn_organization` | `string` | ASN 组织名称 |
| `latitude` / `longitude` | `number` | 经纬度 |
| `timezone` | `string` | 时区 |

下面是完整示例。请将第一段配置为 `ip_script`，第二段配置为 `geo_script`。

```js
function get_ip_by_cf() {
  const urls = [
    "https://1.1.1.1/cdn-cgi/trace",
    "https://[2606:4700:4700::1111]/cdn-cgi/trace",
  ];
  const ipret = [];

  urls.forEach((url) => {
    const content = (get(fetch(url, {
      headers: {
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Safari/537.36",
      },
      retry: 1,
      timeout: 3000,
    }), "body") || "").trim();
    const ip = (content.match(/ip=(\S+)/)?.[1] || "").trim();
    if (ip) ipret.push(ip);
  });

  return ipret;
}

function get_ip_by_ipsb() {
  const urls = ["https://api-ipv4.ip.sb/ip", "https://api-ipv6.ip.sb/ip"];
  const ipret = [];

  urls.forEach((url) => {
    const content = (get(fetch(url, {
      headers: {
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Safari/537.36",
      },
      retry: 1,
      timeout: 3000,
    }), "body") || "").trim();
    if (content !== "") ipret.push(content);
  });

  return ipret;
}

const ip_resolve = get_ip_by_ipsb;
```

```js
const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.61 Safari/537.36";

function handler_ipleak(ip) {
  const content = fetch(`https://ipv4.ipleak.net/json/${ip}`, {
    headers: { "User-Agent": UA },
    retry: 1,
    timeout: 3000,
  });
  const ret = safeParse(get(content, "body"));

  return {
    ip: get(ret, "ip", ""),
    isp: get(ret, "isp_name", ""),
    organization: get(ret, "isp_name", ""),
    latitude: get(ret, "latitude", 0),
    longitude: get(ret, "longitude", 0),
    asn: parseInt(get(ret, "as_number", 0), 10) || 0,
    asn_organization: get(ret, "isp_name", ""),
    timezone: get(ret, "time_zone", ""),
    country: get(ret, "country_name", ""),
    country_code: get(ret, "country_code", ""),
  };
}

function handler(ip) {
  return handler_ipleak(ip);
}
```

示例使用 `ip.sb` 解析出口 IP，并通过 `ipleak.net` 查询地区。可将
`const ip_resolve = get_ip_by_ipsb` 改为 `get_ip_by_cf` 切换 IP 查询服务。

## 开发 Native Handler

Paily 测活后端不内置 native handler。请自行开发检测函数，添加进 `internal/macros/script` 并将配置中的 `deep.script.engine` 设置为 native 或 auto。

```go
package script

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/openpaily/paily-check/internal/engine/request"
	"github.com/openpaily/paily-check/internal/interfaces"
)

func init() {
	RegisterNativeHandler("example", checkExample)
}

func checkExample(ctx context.Context, v interfaces.Vendor, timeoutMS int) (bool, error) {
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	response, _, err := request.Unsafe(requestCtx, v, &interfaces.RequestOptions{
		Method:  http.MethodGet,
		URL:     "https://example.com/health",
		Headers: map[string]string{"Accept": "application/json"},
		Network: interfaces.ROptionsTCP,
	})
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return false, err
	}
	return response.StatusCode == http.StatusOK && string(body) == `{"status":"ok"}`, nil
}
```

配置中，设置 `engine: native` 仅执行已注册的 Native Handler，设置
`engine: auto` 则优先执行 Native Handler，未注册时回退到对应 JS 文件。
