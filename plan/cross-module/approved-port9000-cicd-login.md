# 目标

- 解决本机启动失败：`listen tcp 0.0.0.0:9000: bind`。
- 确认并补齐 GitHub CI/CD 触发方式，避免误把业务登录接口和 GitHub Actions 触发混在一起。

# 范围

- 本机运行态排障：定位并处理占用 `9000` 端口的旧进程。
- 仓库 CI/CD 配置排查：检查 `.github/workflows/*.yml` 的触发事件和必要 secrets。
- 如用户明确要求“业务登录成功后触发 GitHub workflow”，再新增安全受控的 workflow dispatch 方案。

# 改动

- 优先不改代码：先停止占用 `9000` 的旧 Go 运行进程，重新启动服务验证。
- 如只是要“提交/合并后触发 CI/CD”，保留现有 GitHub Actions 触发模型，并仅补充缺失配置或说明。
- 如确认为“应用登录后触发 CI/CD”，按本仓库分层规则新增：
  - 配置结构：GitHub workflow dispatch endpoint、仓库、分支、token 环境变量。
  - `pkg/*`：封装 GitHub Actions dispatch 客户端，不直接依赖业务。
  - `internal/core`：读取配置并初始化客户端。
  - `internal/service/system`：在登录成功后的受控位置编排触发逻辑。
  - `internal/controller/system`：保持只接参、调用 Service、统一响应，不直接调用 GitHub SDK。

# 验证

- `Get-NetTCPConnection -LocalPort 9000` 确认端口释放。
- 启动服务，确认不再出现 `bind` 错误。
- 只读确认 `.github/workflows/ci.yml`、`.github/workflows/cd-prod.yml` 的触发条件。
- 若改 CI/CD 或登录触发逻辑，执行最小必要检查：`go test ./...`，必要时补充针对新增服务/client 的单测。

# 风险

- 直接停止 PID 可能中断你当前正在使用的本机后端实例。
- GitHub Actions 不能由“登录 GitHub 网站”自然触发；它通常由 `push`、`pull_request`、`workflow_dispatch` 等仓库事件触发。
- 业务登录触发 CI/CD 属于敏感能力，必须有开关、鉴权、限流和 token 配置外置，否则会有误触发和凭据泄露风险。

# 执行顺序

1. 确认是否允许停止当前占用 `9000` 的进程 PID `32316`。
2. 停止旧进程后重新检查端口。
3. 重新启动本机服务并确认启动成功。
4. 确认你说的“登录触发 CI/CD”具体含义。
5. 若只是 GitHub 仓库事件触发，核对现有 workflow 并给出需要配置的 secrets 清单。
6. 若确认为业务登录触发 workflow，再进入新的实现计划或在本计划补充后执行。

# 待确认

- 是否允许我停止当前占用 `9000` 的 PID `32316`？
- “登录的时候 GitHub 会触发 CI/CD”具体指哪一种：
  - A. 代码 `push` 到 GitHub 或 PR 时自动触发。
  - B. 合并到 `main` 后自动部署。
  - C. 用户调用本系统登录接口成功后，由后端主动触发 GitHub Actions。
