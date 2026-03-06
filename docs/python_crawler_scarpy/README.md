# crawler_center_scrapy

`crawler_center_scrapy` 是一个基于 FastAPI + Scrapy 的聚合抓取服务，提供统一的 HTTP API，对外暴露稳定的 `/v2/*` 契约。

支持站点：
- LeetCode
- 洛谷（Luogu）
- 蓝桥（Lanqiao）

文档分层：
- 总览与快速使用：本文档
- 完整 API 契约：[docs/api_contract.md](docs/api_contract.md)
- 部署与运维细节：[docs/operations.md](docs/operations.md)


## 1.1 项目能力矩阵

| 能力域 | 当前状态 | 说明 |
| --- | --- | --- |
| LeetCode 抓取 | 已实现 | `profile_meta / recent_ac / submit_stats / public_profile / crawl` |
| Luogu 抓取 | 已实现 | `practice` |
| Lanqiao 抓取 | 已实现 | `solve_stats`（登录 + 抓取一体化） |
| 内部代理池 | 已实现 | `/internal/proxies` 查询、同步、删除；按站点健康管理 |
| 统一错误映射 | 已实现 | 统一失败结构：`{ok:false,error,code}` |
| 观测与日志 | 已实现 | 结构化日志、敏感字段脱敏、请求上下文与错误事件上报 |
| CI/CD | 已实现 | GitHub Actions 进行测试、构建、多架构镜像推送与部署 |


## 1.2 项目特点

- **多站点统一抓取服务**：基于统一 API 规范，聚合 LeetCode / Luogu / Lanqiao 等多平台数据抓取能力，屏蔽上游站点差异，为下游提供稳定可靠的数据源。
- **工程化 CI/CD**：全自动 GitHub Actions 流水线，集成多架构镜像构建 (Buildx)、SHA 版本追溯、健康检查门禁，实现代码提交即刻安全上线，运维零人工干预。
- **高并发异步架构**：深度融合 `FastAPI (ASGI)` 与 `Scrapy (Twisted)` 双异步引擎，突破 Python GIL 限制，单节点轻松支撑数千并发抓取任务。
- **清晰分层设计**：采用 `API -> Service -> Spider -> Parser` 四层解耦模型，业务编排与底层抓取彻底分离，Parser 纯函数化设计确保逻辑 100% 可测试。
- **智能防御与容错**：内置站点级代理池健康管理，自动探测并降级失效节点；全链路异常标准化映射，将复杂的反爬封禁、网络抖动转化为稳定的 HTTP 状态码。
- **全链路测试保障**：集成 `pytest` 测试矩阵，覆盖 API 契约、Service 业务编排、Spider 行为及 Parser 纯函数解析逻辑，重构与迭代零回退风险。
- **生产级可观测性**：默认输出结构化 JSON 日志，敏感数据自动脱敏；内置 `/healthz` 探针与详细错误追踪。



## 2. 1 分钟启动

### 2.1 安装依赖

```bash
python -m venv .venv
# Windows
.venv\Scripts\activate
# macOS / Linux
source .venv/bin/activate
pip install -r requirements.txt
```

### 2.2 启动服务（默认端口 8000）

```bash
uvicorn crawler_center.api.main:app --host 0.0.0.0 --port 8000 --reload
```

Windows 本地开发建议（规避 ProactorEventLoop 与 Twisted reactor 兼容问题）：

```bash
.venv\Scripts\python.exe -m crawler_center.api.run
```

### 2.3 快速验证

```bash
curl http://127.0.0.1:8000/v2/healthz
```

常用入口：
- OpenAPI: `http://127.0.0.1:8000/docs`
- ReDoc: `http://127.0.0.1:8000/redoc`
- Healthz: `http://127.0.0.1:8000/v2/healthz`

## 3. 接口总览

### 3.1 公共接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/v2/healthz` | 服务健康状态 |
| POST | `/v2/leetcode/profile_meta` | LeetCode 用户主页元信息 |
| POST | `/v2/leetcode/recent_ac` | LeetCode 最近 AC |
| POST | `/v2/leetcode/submit_stats` | LeetCode 提交统计 |
| POST | `/v2/leetcode/public_profile` | LeetCode 公开资料 |
| POST | `/v2/leetcode/crawl` | LeetCode 聚合抓取 |
| POST | `/v2/luogu/practice` | Luogu 练题数据 |
| POST | `/v2/lanqiao/solve_stats` | Lanqiao 做题统计 |

### 3.2 内部接口（代理池）

请求头要求：
- `X-Internal-Token: <token>`
- 未配置 token（`INTERNAL_TOKEN`/`config.yaml.internal.token`）返回 `503`
- token 不匹配返回 `401`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/internal/proxies` | 查询代理列表（支持筛选） |
| POST | `/internal/proxies/sync` | 全量替换代理池 |
| POST | `/internal/proxies/remove` | 删除指定代理（`proxy_urls` 数组） |

完整字段与示例见：[docs/api_contract.md](docs/api_contract.md)

## 4. 统一契约

### 4.1 成功与失败结构

成功统一结构：

```json
{
  "ok": true,
  "data": {}
}
```

失败统一结构：

```json
{
  "ok": false,
  "error": "error message",
  "code": "machine_readable_code"
}
```

### 4.2 响应头约定

所有请求会回传：
- `X-Request-ID`
- `X-Trace-ID`

### 4.3 `/v2/healthz` 返回约定

`/v2/healthz` 返回 `data.status`、`data.version`，并包含 `data.observability` 快照字段。

## 5. 配置入口

读取优先级固定为：`env > config.yaml > code defaults`

高频环境变量：

| 变量 | 说明 |
| --- | --- |
| `LEETCODE_BASE_URL` | LeetCode 站点地址 |
| `LUOGU_BASE_URL` | Luogu 站点地址 |
| `LANQIAO_BASE_URL` | Lanqiao 站点地址 |
| `DEFAULT_TIMEOUT_SEC` | 单请求超时（秒） |
| `DEFAULT_SLEEP_SEC` | 默认抓取间隔（秒） |
| `CRAWLER_RUN_TIMEOUT_SEC` | 单次爬虫运行超时（秒） |
| `CRAWLER_CONCURRENT_REQUESTS` | Scrapy 并发请求数 |
| `CRAWLER_RETRY_TIMES` | Scrapy 重试次数 |
| `PROXY_ACTIVE_PROBE_INTERVAL_SEC` | 代理主动探测周期（秒） |
| `INTERNAL_TOKEN` | 内部接口鉴权 token |
| `LOG_LEVEL` | 日志级别 |
| `OBS_ENABLED` | 是否启用观测链路 |
| `OBS_BACKEND` | `redis` / `noop` / `otel`（当前 `otel` 会降级为 `noop`） |
| `OBS_REDIS_URL` | Redis 连接串 |

完整配置矩阵见：
- [docs/api_contract.md](docs/api_contract.md)
- [docs/operations.md](docs/operations.md)

## 6. 内部代理

代理池核心行为：
- 站点维度健康状态：`leetcode / luogu / lanqiao`
- 状态机：`OK -> SUSPECT -> DEAD`
- 被动上报：请求成功/失败后更新健康与延迟
- 主动探测：后台周期性 probe
- 代理池为空时，抓取自动退化为直连

查询过滤规则：
- 支持 `global_status`、`target_site`、`target_status`
- 当传 `target_status` 时，必须同时传 `target_site`（否则返回 `422`）

删除接口请求体（注意字段名）：

```json
{
  "proxy_urls": ["http://127.0.0.1:9000", "http://127.0.0.1:9001"]
}
```

## 7. 可观测性

当前可观测能力：
- 结构化 JSON 日志
- 敏感字段自动脱敏（如 token/password/cookie）
- 请求级上下文（request_id / trace_id）
- 错误事件与请求指标写入器（可采样）

后端说明：
- `OBS_BACKEND=redis`：写入 Redis
- `OBS_BACKEND=noop`：空实现
- `OBS_BACKEND=otel`：当前未实现，会自动降级到 `noop`

## 8. 测试门禁

推荐最小回归：

```bash
.venv\Scripts\python.exe -m pytest -q tests/api tests/services tests/core/test_observability.py
```

全量测试：

```bash
.venv\Scripts\python.exe -m pytest -q
```

说明：当前部分 Windows 环境在 `tests/crawler` 可能出现临时目录权限问题（`PermissionError`），详见 [docs/operations.md](docs/operations.md) 的排障章节。

## 9. 已知限制

- 代理池为进程内内存实现，服务重启后不持久化
- 抓取能力依赖上游站点页面/API 稳定性，上游结构变化时需同步更新 spider/parser
- 当前不提供 `/v2/lanqiao/login`，该路径应返回统一 404 错误结构

---

如果项目对你有帮助，欢迎提交 Issue / PR 交流改进。
