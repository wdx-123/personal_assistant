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




