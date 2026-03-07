# API 契约说明（`/v2/*` + 内部代理接口）

本文档定义当前实现的请求/响应契约，用于客户端对接与回归校验。

基础信息：
- 默认本地地址：`http://127.0.0.1:8000`
- 公共接口前缀：`/v2`
- 内部代理接口前缀：`/internal/proxies`

## 1. 统一响应与头部

### 1.1 成功结构

```json
{
  "ok": true,
  "data": {}
}
```

### 1.2 失败结构

```json
{
  "ok": false,
  "error": "error message",
  "code": "machine_readable_code"
}
```

### 1.3 响应头

所有请求会返回：
- `X-Request-ID`
- `X-Trace-ID`

## 2. 公共接口

## 2.1 GET `/v2/healthz`

说明：健康检查，返回服务状态与版本号。

响应示例：

```json
{
  "ok": true,
  "data": {
    "status": "ok",
    "version": "2.0.0",
    "observability": {
      "status": "ok"
    }
  }
}
```

## 2.2 LeetCode

请求体（复用）：

```json
{
  "username": "demo",
  "sleep_sec": 0.8
}
```

校验：
- `username`: 非空字符串
- `sleep_sec`: `0.0 <= sleep_sec <= 10.0`

### POST `/v2/leetcode/profile_meta`

响应 `data`：
- `meta`: 用户主页元信息（`exists/url_final/og_title/og_description` 等）

### POST `/v2/leetcode/recent_ac`

响应 `data`：
- `recent_accepted`: 列表，元素字段通常包含 `title/slug/timestamp/time`

### POST `/v2/leetcode/submit_stats`

响应 `data`：
- `stats`: 提交统计对象

### POST `/v2/leetcode/public_profile`

响应 `data`：
- `profile`: 用户公开资料对象

### POST `/v2/leetcode/crawl`

响应 `data`：
- `meta`
- `recent_accepted`
- `stats`

## 2.3 Luogu

### POST `/v2/luogu/practice`

请求体：

```json
{
  "uid": 1,
  "sleep_sec": 0.8
}
```

校验：
- `uid > 0`
- `0.0 <= sleep_sec <= 10.0`

响应 `data`（稳定空值兜底）：
- `user`
- `passed`
- `passed_count`

## 2.4 Lanqiao

### POST `/v2/lanqiao/solve_stats`

请求体：

```json
{
  "phone": "13800000000",
  "password": "your-password",
  "sync_num": 0
}
```

校验：
- `phone`: 非空字符串
- `password`: 非空字符串
- `sync_num >= -1`

`sync_num` 规则：
- `-1`：只返回 `stats`
- `0`：返回 `stats + problems`
- `>0`：只返回 `problems`（在前 N 条原始提交范围内筛选并去重）

## 3. 内部代理接口

鉴权规则：
- 必须携带 `X-Internal-Token`
- 若服务未配置 `INTERNAL_TOKEN`（或 `config.yaml.internal.token` 为空），返回 `503`
- token 不匹配返回 `401`

## 3.1 GET `/internal/proxies`

Query 参数：
- `global_status`: `OK | SUSPECT | DEAD`（可选）
- `target_site`: `leetcode | luogu | lanqiao`（可选）
- `target_status`: `OK | SUSPECT | DEAD`（可选）

约束：
- 提供 `target_status` 时，必须同时提供 `target_site`，否则返回 `422`

响应 `data`：
- `items`: 代理条目列表
- `summary.total`
- `summary.by_global_status`
- `summary.applied_filters`

## 3.2 POST `/internal/proxies/sync`

请求体：

```json
{
  "proxies": ["http://127.0.0.1:9000", "http://127.0.0.1:9001"]
}
```

响应 `data`：
- `total`
- `added`
- `updated`
- `removed`

## 3.3 POST `/internal/proxies/remove`

请求体（字段名固定为 `proxy_urls`）：

```json
{
  "proxy_urls": ["http://127.0.0.1:9000"]
}
```

响应 `data`：
- `total`
- `removed`

## 4. 错误码映射

| HTTP 状态 | code | 说明 |
| --- | --- | --- |
| 401 | `http_error` | 内部 token 鉴权失败 |
| 422 | `validation_error` | 请求参数校验失败 |
| 502 | `upstream_request_error` | 上游请求失败 |
| 502 | `crawler_execution_error` | 爬虫执行失败（非超时） |
| 503 | `proxy_unavailable` | 代理不可用 |
| 503 | `http_error` | 内部 token 未配置（通过 HTTPException 返回） |
| 504 | `crawler_timeout` | 爬虫执行超时 |
| 500 | `internal_error` | 未捕获异常 |

## 5. 已移除接口说明

- `/v2/lanqiao/login` 当前不存在，访问应返回：
  - HTTP `404`
  - 统一失败结构：`{ok:false,error,code:"http_error"}`

## 6. 配置矩阵（完整）

优先级：`env > config.yaml > defaults`

### 6.1 核心抓取与 API

| 环境变量 | 说明 |
| --- | --- |
| `LEETCODE_BASE_URL` | LeetCode base URL |
| `LUOGU_BASE_URL` | Luogu base URL |
| `LANQIAO_BASE_URL` | Lanqiao base URL |
| `LANQIAO_LOGIN_URL` | Lanqiao 登录 URL |
| `LANQIAO_USER_URL` | Lanqiao 用户信息 URL |
| `DEFAULT_TIMEOUT_SEC` | 单请求超时（秒） |
| `DEFAULT_SLEEP_SEC` | 默认 sleep（秒） |
| `DEFAULT_USER_AGENT` | 默认 UA |
| `API_TITLE` | FastAPI title |
| `API_VERSION` | FastAPI version |
| `CRAWLER_RUN_TIMEOUT_SEC` | 单次 runner 超时 |
| `CRAWLER_CONCURRENT_REQUESTS` | Scrapy 并发 |
| `CRAWLER_RETRY_TIMES` | Scrapy 重试次数 |
| `PROXY_ACTIVE_PROBE_INTERVAL_SEC` | 代理主动探测间隔 |
| `INTERNAL_TOKEN` | 内部鉴权 token |
| `LOG_LEVEL` | 日志级别 |

### 6.2 Observability

| 环境变量 | 说明 |
| --- | --- |
| `OBS_ENABLED` | 是否启用观测 |
| `OBS_BACKEND` | `redis/noop/otel`（`otel` 当前降级 `noop`） |
| `OBS_ENV` / `APP_ENV` | 观测环境名 |
| `OBS_SERVICE_NAME` | 服务名 |
| `OBS_SERVICE_INSTANCE` | 服务实例标识 |
| `OBS_GRAY_PERCENT` | 采样百分比（0-100） |
| `OBS_GRAY_ROUTE_ALLOWLIST` | 路由白名单（逗号分隔） |
| `OBS_REDIS_URL` | Redis URL |
| `OBS_STREAM_ERRORS_KEY` | 错误 stream key |
| `OBS_STREAM_METRICS_KEY` | 指标 stream key |
| `OBS_STREAM_ERRORS_DLQ_KEY` | 错误 DLQ key |
| `OBS_STREAM_METRICS_DLQ_KEY` | 指标 DLQ key |
| `OBS_TRACE_KEY_PREFIX` | trace key 前缀 |
| `OBS_TRACE_TTL_SEC` | trace TTL |
| `OBS_TRACE_MAX_EVENTS` | trace 最大事件数 |
| `OBS_RETRY_MAX` | 写入重试次数 |
| `OBS_RETRY_BASE_MS` | 重试退避基线（ms） |
| `OBS_RETRY_MAX_MS` | 重试退避上限（ms） |
| `OBS_OUTBOX_ENABLED` | 是否启用 outbox |
| `OBS_OUTBOX_DIR` | outbox 目录 |
| `OBS_OUTBOX_MAX_BYTES` | outbox 最大大小 |
| `OBS_FAIL_OPEN` | 观测失败是否不阻断主流程 |
| `OBS_METRIC_BUFFER_MAX` | 指标缓冲上限 |
| `OBS_ERROR_MESSAGE_MAX_LEN` | 错误消息最大长度 |
