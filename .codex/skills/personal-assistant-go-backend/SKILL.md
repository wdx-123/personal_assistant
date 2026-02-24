---
name: personal-assistant-go-backend
description: 用于 personal_assistant 仓库的 Go 后端开发、重构、代码评审与排障。任务涉及分层实现、DTO到Router链路、BizError错误处理、context传递、路由注册、配置外置和规则合规检查时使用。
---

# Personal Assistant Go Backend

## 概览

使用本 skill 在当前仓库执行后端任务，确保规则一致、交付稳定、实现路径可复用。

先阅读 `references/project-rules.md` 作为权威规则，再按下方流程选择执行路径。

## 工作流决策树

1. 用户要求新增或修改 API/业务逻辑：
   - 按标准实现流程执行（DTO 到 Router）。
2. 用户要求做代码评审：
   - 按评审清单执行，并按严重级别输出问题。
3. 用户要求排障或重构：
   - 先定位受影响层，再在不破坏边界的前提下做最小改动。

## 标准实现流程

### 第 1 步：DTO

- 在以下目录定义请求和响应 DTO：
  - `internal/model/dto/request`
  - `internal/model/dto/response`
- 禁止直接将 Entity 作为 API 契约。
- 校验规则与字段语义需和 Controller 绑定逻辑一致。

### 第 2 步：Repository

- 在 `internal/repository/interfaces` 扩展仓储接口。
- 在 `internal/repository/system` 实现数据访问。
- Repository 不感知业务语义与 BizError。
- 原样返回底层错误。

### 第 3 步：Service

- 在 `internal/service/system` 编排业务逻辑。
- 持久化只通过 Repository。
- 业务错误使用 `pkg/errors` 包装。
- 长链路调用必须贯穿 `context.Context`。

### 第 4 步：Controller

- 完成请求绑定与参数校验。
- 使用请求上下文调用 Service。
- 失败统一 `global.Log.Error` 记录。
- 统一使用 `pkg/response` 返回。
- BizError 路径使用 `response.BizFailWithError(err, c)`。

### 第 5 步：Router

- 在 `internal/router/system` 注册模块路由。
- 每个模块仅提供一个 `InitXRouter(*gin.RouterGroup)`。
- 统一接入中央路由组注册流程。

### 第 6 步：收尾检查

- 检查是否存在分层越界。
- 检查配置是否通过 `global.Config` 外置读取。
- 检查错误链路是否统一。
- 至少执行最小必要测试或校验命令。

## 禁止模式

- 在 Controller 写业务逻辑。
- Service 直接访问 DB 或 `global.DB`。
- Controller 直接调用 Repository。
- 业务层直接读 Viper。
- 绕过 `pkg/response` 返回散装响应。
- 硬编码可变运行参数。

## 仓库导航

- 启动编排：`internal/init/init.go`
- 核心初始化：`internal/core`
- 路由入口：`internal/router/router.go`
- 领域路由注册：`internal/router/system`
- Controller：`internal/controller/system`
- Service：`internal/service/system`
- Repository：`internal/repository/system`
- Repository 契约：`internal/repository/interfaces`
- DTO：`internal/model/dto`
- 配置结构体：`internal/model/config`
- 公共能力：
  - `pkg/errors`
  - `pkg/response`
  - `pkg/jwt`
  - `pkg/util`

## 评审执行规则

- 当用户要求 “review” 时，重点关注：
  - 缺陷
  - 行为回归
  - 测试缺口
  - 分层越界
- 先输出问题项，并附文件与行号。
- 总结放在后面。
- 若无发现，明确说明并给出剩余风险。

## 交付前清单

- 规则已按 `references/project-rules.md` 对齐。
- 受影响模块已按 `references/repo-map.md` 定位。
- 已执行 `references/review-checklist.md`。
- 可选执行静态探针 `scripts/rule_probe.ps1`。
- 需要时执行 `scripts/install_skill.ps1` 完成本机安装/同步。
