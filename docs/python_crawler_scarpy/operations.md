# 运维与发布说明

本文档聚焦部署、发布、回滚与常见排障。

## 1. 本地运行与容器运行

## 1.1 本地运行

```bash
uvicorn crawler_center.api.main:app --host 0.0.0.0 --port 8000
```

Windows 建议：

```bash
.venv\Scripts\python.exe -m crawler_center.api.run
```

原因：Windows 默认 ProactorEventLoop 与 Scrapy/Twisted `AsyncioSelectorReactor` 存在兼容问题。

## 1.2 Docker 本地运行

```bash
docker build -t ghcr.io/wdx-123/crawler_center_scrapy:local .
docker run --rm -p 8000:8000 -v $(pwd)/config.yaml:/app/config.yaml:ro ghcr.io/wdx-123/crawler_center_scrapy:local
```

Windows PowerShell：

```powershell
docker run --rm -p 8000:8000 -v ${PWD}\config.yaml:/app/config.yaml:ro ghcr.io/wdx-123/crawler_center_scrapy:local
```

健康检查：

```bash
curl http://127.0.0.1:8000/v2/healthz
```

## 2. Compose 部署（服务器）

假设部署目录为 `/opt/crawler_center_scrapy`：

```bash
mkdir -p /opt/crawler_center_scrapy
cd /opt/crawler_center_scrapy
```

准备文件：
- `config.yaml`（必须）
- `docker-compose.yml`
- `.env`（可由 `.env.example` 初始化）

```bash
cp .env.example .env
```

示例 `.env`：

```env
IMAGE_TAG=latest
CONFIG_PATH=/opt/crawler_center_scrapy/config.yaml
TZ=Asia/Shanghai
LOG_LEVEL=INFO
INTERNAL_TOKEN=replace-with-your-token
OBS_ENABLED=false
OBS_BACKEND=redis
OBS_GRAY_PERCENT=0
OBS_GRAY_ROUTE_ALLOWLIST=
OBS_REDIS_URL=redis://127.0.0.1:6379/0
```

启动：

```bash
docker compose pull
docker compose up -d
```

验证：

```bash
curl http://127.0.0.1:8000/v2/healthz
docker compose logs --tail=100
```

## 3. GitHub Actions 发布与自动部署

对应工作流：`.github/workflows/cd.yml`

触发：
- `push` 到 `main`
- `workflow_dispatch`

行为：
- 运行测试（`tests/api tests/crawler tests/services`）
- 构建并推送多架构镜像（`linux/amd64` + `linux/arm64`）
- 使用 `sha-<short_sha>` 部署到服务器
- 健康检查失败时输出容器日志并让流程失败

## 4. GitHub Secrets / Variables 对齐清单

## 4.1 必填 Secrets

- `DEPLOY_HOST`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `GHCR_USERNAME`
- `GHCR_TOKEN`

说明：
- 工作流当前会在服务器端执行 `docker login ghcr.io`，必须提供 `GHCR_USERNAME/GHCR_TOKEN`
- `DEPLOY_HOST_FINGERPRINT` 在当前工作流中处于注释状态，不是必填

## 4.2 可选 Variables（有默认值）

- `DEPLOY_PORT`（默认 `22`）
- `DEPLOY_PATH`（默认 `/opt/crawler_center_scrapy`）
- `HEALTHCHECK_URL`（默认 `http://127.0.0.1:8000/v2/healthz`）

## 5. 回滚

把 `.env` 中 `IMAGE_TAG` 改为历史版本（例如 `sha-xxxxxxx`），然后重启：

```bash
docker compose up -d
```

## 6. 可观测性运维说明

关键变量：
- `OBS_ENABLED`
- `OBS_BACKEND`
- `OBS_REDIS_URL`

后端行为：
- `OBS_BACKEND=redis`：启用 Redis 写入
- `OBS_BACKEND=noop`：不写入外部后端
- `OBS_BACKEND=otel`：当前未实现，会自动降级为 `noop`

## 7. 常见排障

## 7.1 Windows 启动报 reactor/loop 相关错误

优先使用：

```bash
.venv\Scripts\python.exe -m crawler_center.api.run
```

避免直接在不兼容事件循环下启动导致 Scrapy runner 异常。

## 7.2 `tests/crawler` 在 Windows 出现临时目录权限错误

现象：
- 运行 `tests/crawler` 可能报 `PermissionError`（常见于系统临时目录无写权限）

建议：
- 先执行契约相关最小回归：

```bash
.venv\Scripts\python.exe -m pytest -q tests/api tests/services tests/core/test_observability.py
```

- 若需跑 `tests/crawler`，先确认临时目录可写权限后再执行。
