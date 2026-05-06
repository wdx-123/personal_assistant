## 如何将项目从MVC改造至DDD架构

如果要设计一个，原生的DDD架构，最起初的时候，

```go
internal/
  core/                         # 继续只放初始化/装配，不放 AI 领域协议

  ai/
    interfaces/                 # 对外入口层
      http/
        controller.go           # 从 internal/controller/system/aiCtrl.go 迁移
        router.go               # 从 internal/router/system/aiRouter.go 迁移
        mapper.go               # HTTP DTO <-> application command/query 转换

    application/                # 应用用例层
      service.go                # 从 aiSvc.go 拆出用例编排
      command.go                # 创建会话、流式对话、提交决策等命令
      query.go                  # 会话列表、消息列表等查询
      planner.go                # 从 aiPlanner.go 迁移，先作为应用策略
      context_resolver.go       # 从 aiContext.go 迁移
      projector.go              # 从 aiProjector.go 迁移
      recovery_service.go       # 从 aiControlPlane_runtime.go 中恢复编排迁移
      factory.go                # 从 aiRuntimeFactory.go 迁移
 
    domain/                     # AI 领域核心
      runtime.go                # AIRuntime / AIRuntimeRecoverer
      sink.go                   # AIRuntimeSink
      plan.go                   # AIRuntimePlan / ToolBlueprint / ExecutionInput
      event.go                  # runtime event name / event payload 语义
      interrupt.go              # interrupt 状态、RuntimeState
      decision.go               # confirm / skip / DecisionCommand
      conversation.go           # AIConversation 领域对象，可先轻量封装
      message.go                # AIMessage 领域对象，可先轻量封装
      errors.go                 # AI 子域领域错误语义，可选

    infrastructure/             # 技术实现层
      runtime/
        local/
          runtime.go            # 从 aiRuntimeLocal.go 迁移
        eino/
          runtime.go            # 从 aiRuntimeEino.go 迁移
          agent_factory.go      # 从 infrastructure/ai/eino/agent_factory.go 迁移
          approval_middleware.go
          checkpoint_store.go
          docs_tool.go
          models.go
          task_progress_tools.go

      persistence/
        gorm_repository.go      # 从 internal/repository/system/aiRepo.go 迁移
        model.go                # 可选：AI 专属 GORM persistence model

      runtimecontrol/
        redis_command_bus.go    # 从 infrastructure/ai/runtimecontrol 迁移
        redis_envelope_store.go
        redis_recovery_lock.go  # 后续可拆

      sse/
        stream_sink.go          # 从 aiSink.go 迁移
        http_writer_adapter.go  # 如需隔离 HTTP SSE writer，可新增

    dto/                        # AI 子域接口 DTO，可选
      request.go                # 从 internal/model/dto/request/aiReq.go 迁移
      response.go               # 从 internal/model/dto/response/aiResp.go 迁移

```

但其实，**DDD 不要求你必须把目录命名成 interfaces/application/domain/infrastructure。**
DDD 更重要的是依赖方向和职责边界：

- controller/router 就是对外入口层，不需要改名成 interfaces。
- service/system 就是应用层，不需要改名成 application。
- 新增 domain/ai 放 AI 子域稳定协议。
- infrastructure/ai 放 Eino、Redis control、LLM gateway、mem0 这类技术实现。

这样就是“局部 DDD”，但仍然保留项目当前 MVC 项目的整体风格。

```go
internal/
  core/                  # 初始化/装配，继续不动

  controller/
    system/
      aiCtrl.go          # AI HTTP 入口

  router/
    system/
      aiRouter.go        # AI 路由入口

  service/
    system/
      aiSvc.go           # AI 应用编排层
      aiPlanner.go
      aiProjector.go
      aiContext.go
      aiRuntimeFactory.go

  domain/
    ai/
      runtime.go         # AIRuntime 接口
      sink.go            # AIRuntimeSink 接口
      plan.go            # Plan / ExecutionInput
      event.go           # Runtime 事件定义
      interrupt.go       # Interrupt 状态/协议
      decision.go        # 用户决策协议

  infrastructure/
    ai/
      eino/              # Eino runtime 实现
      runtimecontrol/    # Redis 控制面 / recovery / command bus

  repository/
    interfaces/
      aiRepository.go
    system/
      aiRepo.go

```
因为在这次的项目中，
并不是为了用 DDD 推翻现有 MVC，而只需要给 AI 子域补一个 domain/ai 稳定核心层，让 Service 和 Infrastructure 都围着它依赖。

## 当前项目正式口径

当前项目对外和对内的统一表述，应当是：

> **项目整体仍是传统 MVC 主体架构，针对 AI 核心子域做渐进式 DDD 分层改造。**

这句话的含义是：

- 不是说整个项目已经完成全量 DDD 重构。
- 也不是说 AI 子域仍然应该继续全部堆在单个 MVC Service 里。
- 而是：
  - `controller/router/service/repository` 主体结构继续保留；
  - 当 AI 逻辑出现稳定协议、事件语义、可替换 runtime、tool 抽象、恢复控制等需求时，
    再把这些内容渐进式收口到 `domain/ai` 与 `infrastructure/ai`。

因此，后续在本项目做 AI 改动时，默认优先遵守下面这条落点规则：

- 稳定协议、事件、tool/runtime 抽象：放 `internal/domain/ai`
- Eino / Local runtime、模型 SDK、第三方 Agent 适配：放 `internal/infrastructure/ai`
- 会话流程、上下文组装、tool 注册授权、trace/projector 协调：放 `internal/service/system`
- HTTP / SSE 入口：继续放 `internal/controller/system` 与 `internal/router/system`

这就是“AI 子域局部 DDD”，不是“项目整体目录全面改名”。



我能不能在拆分的时候，先把这部分给踢掉，等后期在新添加上去，顺便把A2UI这个踢掉，
	
