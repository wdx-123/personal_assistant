你可以把这套 AI 架构理解成：**它不是一开始就设计得这么复杂，而是被需求一步步“逼”出来的。**

## 1. 最初：普通 API 问答

最早的形态可以很简单：

```text
用户发问题
-> Controller 接收
-> Service 调用大模型 API
-> 返回字符串
```

这时候 MVC 完全够用。

对应心智是：

```text
AICtrl -> AIService -> LLM API -> response
```

问题也很明显：

- 模型响应慢，用户只能等。
- 没有流式体验。
- 中间过程不可见。
- 没有工具调用。
- 没有消息状态。
- 没有中断、恢复、确认。

所以这个阶段只需要 `Controller + Service`。

---

## 2. 后来：为了体验，引入 SSE

模型回答可能很长，所以你引入 SSE。

链路变成：

```text
用户发问题
-> Controller 创建 SSE writer
-> Service 调用模型
-> 一边生成一边推给前端
```

这时就多了一个问题：

> 模型输出不再是一次性字符串，而是一串事件。

所以你需要一个事件出口，也就是现在的：

```text
AIRuntimeSink
aiStreamSink
```

它的出发点是：

> Runtime 不应该直接碰 HTTP ResponseWriter，而是只发事件。

对应现在的代码：

- [aiCtrl.go](d:/workspace_go/test/go/personal_assistant/internal/controller/system/aiCtrl.go)
- [aiSink.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiSink.go)

---

## 3. 再后来：为了历史消息，需要落库

SSE 解决了实时体验，但还不够。

你还需要：

- 会话列表
- 历史消息
- 生成状态
- 错误状态
- 工具轨迹
- 等待确认状态

所以就有了：

```text
AIConversation
AIMessage
AIInterrupt
```

对应职责：

```text
AIConversation 记录会话
AIMessage      记录用户消息和 assistant 消息
AIInterrupt    记录等待确认 / 恢复信息
```

这时 `aiStreamSink` 不能只写 SSE，它还要同步写 DB。

于是出现了：

```text
runtime event
-> aiStreamSink
   -> SSE
   -> aiMessageProjector
      -> AIRepository
      -> DB
```

`aiProjector` 的出发点是：

> 把运行时事件折叠成数据库里的消息快照。

例如：

```text
assistant_token -> 追加 Content
tool_call_started -> 更新 trace_items
structured_block -> 更新 ui_blocks
message_completed -> 标记 success
error -> 标记 error
```

对应文件：

- [aiProjector.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiProjector.go)
- [aiRepo.go](d:/workspace_go/test/go/personal_assistant/internal/repository/system/aiRepo.go)
- [ai.go](d:/workspace_go/test/go/personal_assistant/internal/model/entity/ai.go)

---

## 4. 再后来：为了让 Agent 查业务数据，引入 Tool

普通 LLM 只能回答它知道的东西。  
但你的系统里有真实业务数据：

- 当前用户
- 当前组织
- OJ 任务
- 训练进度
- 项目文档

所以你引入工具：

```text
get_task_snapshot
get_progress_snapshot
search_project_docs
```

这时模型不是单纯生成文本，而是：

```text
读用户问题
-> 判断需要哪些工具
-> 调工具拿业务数据
-> 再生成回答
```

所以你需要 `aiContext.go`：

```text
把系统里的真实业务数据整理成 AI 可用上下文
```

比如：

- `ResolveContext`
- `TaskSnapshot`
- `ProgressSnapshot`

对应文件：

- [aiContext.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go)
- [task_progress_tools.go](d:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/task_progress_tools.go)
- [docs_tool.go](d:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/docs_tool.go)

---

## 5. 再后来：为了控制模型乱调用，引入 Plan

如果完全让大模型自由决定工具，它可能：

- 明明不该查文档却查文档
- 重复调用工具
- 越过业务边界
- 调用不存在的工具
- 在不需要复杂流程时也走重链路

所以你加了 `plan`。

`plan` 的出发点是：

> 在真正执行前，先由业务侧决定本轮允许做什么。

现在链路是：

```text
用户问题
-> planAIRuntime
-> AIRuntimePlan
-> Runtime 按 plan 执行
```

`AIRuntimePlan` 里会表达：

```text
是否 lightweight
是否需要 task tool
是否需要 progress tool
是否需要 docs tool
是否需要展示 thinking summary
最终回答 confirm / skip 分支
```

对应文件：

- [aiPlanner.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiPlanner.go)
- [aiIntent.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiIntent.go)
- [aiRuntime.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiRuntime.go)

---

## 6. 再后来：为了安全控制，引入 Interrupt / Decision / Resume

文档工具、未来的记忆工具、跨组织查询、发送通知、批量变更，这些都不能让模型无限制调用。

所以出现了：

```text
interrupt
decision
resume
```

它的出发点是：

> 模型准备做一个有风险或需要确认的动作时，先暂停，等用户确认后再继续。

现在文档工具的链路是：

```text
Eino 准备调用 search_project_docs
-> ApprovalMiddleware 打断
-> Runtime 发送 tool_call_waiting_confirmation
-> 用户 confirm / skip
-> SubmitDecision
-> Runtime Resume
-> 继续生成结果
```

这就是为什么你现在需要：

```text
AIInterrupt
RuntimeStateJSON
checkpoint_id
resume_target_id
decision
reason
owner_node_id
```

对应文件：

- [approval_middleware.go](d:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/approval_middleware.go)
- [aiControlPlane.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiControlPlane.go)
- [aiControlPlane_runtime.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiControlPlane_runtime.go)

---

## 7. 再后来：为了替换实现，引入 Runtime 抽象

一开始你可能只接一个模型 API。  
后来你接了 Eino。  
未来你还可能接：

- LLM Gateway
- mem0
- LangGraph 类框架
- 自研 Agent Runner
- 多模型路由

所以你不能让 `AIService` 直接依赖某个具体框架。

于是有了：

```go
AIRuntime
```

它的出发点是：

> AIService 不关心底层用 Eino、Local、网关还是其他 Agent 框架，只关心这轮对话能不能 Plan、Execute、SubmitDecision。

当前有两个实现：

```text
LocalAIRuntime  本地模拟 / fallback / test
EinoAIRuntime   正式 Eino 执行引擎
```

对应文件：

- [aiRuntime.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiRuntime.go)
- [aiRuntimeLocal.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiRuntimeLocal.go)
- [aiRuntimeEino.go](d:/workspace_go/test/go/personal_assistant/internal/service/system/aiRuntimeEino.go)

---

## 8. 再后来：为了多节点和恢复，引入 Runtime Control Plane

如果只有单机，一次 SSE 等待用户确认还好。  
但如果未来多节点部署，就会出现问题：

```text
用户的 stream 在 A 节点
decision 请求打到 B 节点
```

这时 B 节点怎么把决策送回 A 节点？

所以你有了：

```text
runtime command bus
runtime envelope
owner lease
recovery loop
```

它的出发点是：

> 记录当前 interrupt 属于哪个 runtime 节点，并在 owner 丢失时恢复或安全停止。

对应组件：

```text
RedisCommandBus
RedisEnvelopeStore
RecoveryLock
```

对应文件：

- [redis_command_bus.go](d:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/runtimecontrol/redis_command_bus.go)
- [redis_envelope_store.go](d:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/runtimecontrol/redis_envelope_store.go)

---

## 各包的出发点

```text
controller/system
```

出发点：HTTP 入口。  
只负责绑定参数、拿用户 ID、创建 SSE writer、返回响应。

```text
service/system
```

出发点：业务用例编排。  
决定这个用户能不能跑、怎么落库、什么时候调用 runtime、怎么收尾。

```text
service/system/aiRuntime.go
```

出发点：抽象 AI 执行引擎。  
隔离 Service 和具体 Agent 框架。

```text
service/system/aiSink.go
```

出发点：隔离 runtime 和输出通道。  
Runtime 只发事件，Sink 决定写 SSE 和 DB。

```text
service/system/aiProjector.go
```

出发点：把事件变成可恢复的消息状态。  
保证实时流和历史消息一致。

```text
service/system/aiPlanner.go
```

出发点：限制和规划工具调用。  
不要让大模型无限制自由决定流程。

```text
service/system/aiContext.go
```

出发点：给 Agent 提供真实业务上下文。  
模型不能直接查 DB，只能通过业务裁剪后的上下文和工具。

```text
infrastructure/ai/eino
```

出发点：技术实现适配。  
这里是 Eino、Agent、Tool、Checkpoint、ApprovalMiddleware。

```text
infrastructure/ai/runtimecontrol
```

出发点：运行控制面。  
解决跨节点 decision 路由、owner lease、recovery。

```text
repository
```

出发点：数据库访问。  
只负责 conversation、message、interrupt 的 CRUD / lock / query。

---

## 为什么最后会变成现在这样

因为 AI 模块经历了这条演进线：

```text
同步问答
-> 流式输出
-> 会话与消息落库
-> Agent 工具调用
-> 工具调用规划
-> 用户确认 interrupt
-> checkpoint resume
-> 多节点控制面
-> runtime 抽象
```

所以它自然从普通 MVC 的：

```text
Controller -> Service -> Repository
```

演进成了：

```text
Controller
  -> Service
    -> Runtime
      -> Sink
        -> SSE
        -> Projector
          -> Repository
```

这不是为了炫技，也不是为了强行 DDD。

而是因为你的 AI 模块已经有了普通 CRUD 没有的东西：

- 长连接
- 流式事件
- 工具调用
- 运行计划
- 用户确认
- 中断恢复
- 多节点 owner
- 历史消息投影
- 可替换 AI 框架

所以它需要一个比普通 Service 更清晰的执行边界。

一句话：

**你的 AI 架构是从“调一次模型 API”一步步演进到“可恢复、可控制、可落库、可替换运行时的 Agent 执行系统”。**