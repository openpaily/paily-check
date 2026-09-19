# 测活流程

本文档介绍 Paily 测活后端的初筛与复筛流程、各阶段参数，以及上报结果的字段含义。

## 总体流程

一轮测活从 Paily 主服务拉取节点后，按以下顺序执行：

```text
所有节点
  │
  ├─ 初筛：延迟检测
  │    └─ 延迟为 -1 或超过阈值的节点停止后续检测
  │
  └─ 存活节点
       ├─ 详细延迟：平均延迟与抖动
       ├─ 出口地区
       ├─ 流媒体脚本
       └─ 下载测速（可选）
            │
            ▼
          汇总并上报
```

各阶段只运行一种测试类型，逐阶段串行推进，阶段内部按配置的并发数执行。

## 初筛

初筛对所有节点做少量 Ping，目标是快速筛掉不可达的节点。

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | ---: | --- |
| `initial.ping_url` | `INITIAL_PING_URL` | `https://www.gstatic.com/generate_204` | 初筛 Ping 地址。 |
| `initial.ping_avg` | `INITIAL_PING_AVG` | `3` | 每个节点的 Ping 次数。 |
| `initial.ping_timeout_ms` | `INITIAL_PING_TIMEOUT_MS` | `3000` | 单次 Ping 超时，单位毫秒。 |
| `initial.concurrency` | `INITIAL_CONCURRENCY` | `50` | 初筛并发数。 |
| `initial.alive_threshold_ms` | `INITIAL_ALIVE_THRESHOLD_MS` | `3000` | 超过该延迟的节点不再进入复筛；设为 `0` 表示除不可达外全部继续。 |

延迟取所有 Ping 的平均值。若某次 Ping 超时，按超时时间计入平均；若全部超时，延迟记为 `-1`。

## 复筛

复筛对初筛存活的节点执行四类检测。复筛不会再次筛选节点，而是评估它们的详细表现。

### 详细延迟

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | ---: | --- |
| `deep.ping.concurrency` | `DEEP_PING_CONCURRENCY` | `20` | 复筛并发数。 |
| `deep.ping.avg` | `DEEP_PING_AVG` | `7` | 每个节点的 Ping 次数。 |
| `deep.ping.ping_url` | `DEEP_PING_URL` | `https://www.gstatic.com/generate_204` | 复筛 Ping 地址。 |
| `deep.ping.timeout_ms` | `DEEP_PING_TIMEOUT_MS` | `3000` | 单次 Ping 超时，单位毫秒。 |
| `deep.ping.filter_dead_deep` | `DEEP_PING_FILTER_DEAD` | `false` | 是否丢弃详细延迟仍为 `-1` 的节点，使其不进入后续阶段。 |

该阶段记录平均延迟和抖动。超时样本同样计入平均值。

### 出口地区

出口地区检测节点的实际出口国家或地区，支持三种模式。

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | ---: | --- |
| `deep.geo.concurrency` | `DEEP_GEO_CONCURRENCY` | `20` | 地区检测并发数，建议不超过 `50`。 |
| `deep.geo.mode` | `GEO_MODE` | `script` | `script`、`mmdb` 或 `native_api`。 |
| `deep.geo.mmdb_path` | `GEO_MMDB_PATH` | 空 | `mode=mmdb` 时必填。程序会在启动及之后每 12 小时刷新该文件。 |
| `deep.geo.fallback_legacy` | `GEO_FALLBACK_LEGACY` | `true` | 内置模块无有效结果时，是否回退到其他检测逻辑。 |
| `deep.geo.fallback_inbound` | `GEO_FALLBACK_INBOUND` | `false` | 全部出口检测失败时，是否直接查询节点配置的服务器地址（该结果不是出口地区）。 |
| `deep.geo.retry` | `GEO_RETRY` | `2` | IP 与地区查询失败时的重试轮数。 |
| `deep.geo.ip_script` | `GEO_IP_SCRIPT` | 空 | 自定义出口 IP 查询脚本；为空时使用内置实现。 |
| `deep.geo.geo_script` | `GEO_GEO_SCRIPT` | 空 | 自定义 GeoIP 查询脚本；为空时使用内置实现。 |
| `deep.geo.native_api.timeout_ms` | `GEO_NATIVE_API_TIMEOUT_MS` | `5000` | `native_api` 模式下单节点并发查询的总超时。 |
| `deep.geo.native_api.providers` | `GEO_NATIVE_API_PROVIDERS` | 内置提供方列表 | 启用的公共 GeoIP API 提供方，逗号分隔。 |

自定义出口 IP 和 GeoIP 脚本的开发规范见 [SCRIPTING.md](../SCRIPTING.md)。

### 流媒体脚本

该阶段运行解锁检测脚本，结果是一个「服务名 → 是否解锁」的映射。

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | ---: | --- |
| `deep.script.node_concurrency` | `DEEP_SCRIPT_NODE_CONCURRENCY` | `5` | 同时处理的节点数。 |
| `deep.script.script_concurrency` | `DEEP_SCRIPT_CONCURRENCY` | `32` | 所有节点共享的脚本执行并发上限。 |
| `deep.script.dir` | `SCRIPT_DIR` | `./scripts` | 检测脚本目录。 |
| `deep.script.timeout_ms` | `SCRIPT_TIMEOUT_MS` | `10000` | 单个脚本的执行超时，单位毫秒。 |
| `deep.script.engine` | `SCRIPT_ENGINE` | `goja` | `goja`（JS 脚本）、`native`（编译进二进制的逻辑）或 `auto`（优先 native，回退 JS）。 |
| `deep.script.regions` | `SCRIPT_REGIONS` | 空 | 仅在这些出口地区执行脚本，逗号分隔；为空表示不筛选。 |
| `deep.script.scripts` | `SCRIPT_LIST` | 空 | 仅加载指定脚本文件名，逗号分隔；为空表示加载目录内全部脚本。 |

脚本编写方法见 [SCRIPTING.md](../SCRIPTING.md)。

### 下载测速

下载测速默认关闭，启用后测量节点的下载速度。

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | ---: | --- |
| `deep.speed.enabled` | `SPEED_ENABLED` | `false` | 是否启用下载测速。 |
| `deep.speed.concurrency` | `DEEP_SPEED_CONCURRENCY` | `2` | 测速并发数，建议保持较低。 |
| `deep.speed.url` | `SPEED_URL` | 内置测速地址 | 测速目标 URL，启用测速时必须设置。 |
| `deep.speed.threads` | `SPEED_THREADS` | `4` | 下载线程数。 |
| `deep.speed.duration_s` | `SPEED_DURATION_S` | `8` | 单次测速持续时间，单位秒。 |

未启用测速时，上报结果中不包含速度字段，Paily 主服务在评分时也不会使用速度。

## 调度与进度

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `schedule.cron` | `CHECK_CRON` | `0 */30 * * * *` | 六字段 Cron 表达式：秒 分 时 日 月 周。 |
| `progress_bar.enabled` | `PROGRESS_BAR` | `false` | 是否在终端显示实时进度条。 |

启动时会立即执行一轮，之后按 Cron 触发。上一轮未结束时，新的触发会被跳过。每个阶段结束时都会输出耗时与每秒处理节点数。

## 上报字段

每个节点上报一条结果：

| 字段 | 说明 |
| --- | --- |
| `node_id` | 节点 ID。 |
| `initial.latency_ms` | 初筛延迟；`-1` 表示不可达。 |
| `deep.avg_latency_ms` | 复筛平均延迟。 |
| `deep.jitter_ms` | 复筛抖动。 |
| `deep.speed_kbps` | 下载速度；未测速时不包含该字段。 |
| `deep.region` | 出口地区代码。 |
| `deep.streaming` | 流媒体解锁结果，键为服务名，值为是否解锁。 |
| `deep.metadata` | 其他元数据。 |

未通过初筛的节点只上报 `initial`，不包含 `deep`。
