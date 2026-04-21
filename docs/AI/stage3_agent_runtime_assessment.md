# 项目当前阶段判断

- 本结论综合了 4 个并行 `gpt-5.4` 子任务结果：公开资料研究、本地代码审计、下一阶段方案设计、架构与面试包装；并结合本地只读验证 `go test ./internal/service/system -count=1`、`go test ./internal/infrastructure/ai/eino -count=1`。
- 当前项目已进入“第二阶段正式运行时骨架基本完成，但第三阶段生产级闭环尚未完成”的状态。更准确地说，它已经不是 `LocalAIRuntime` demo，而是以 `EinoAIRuntime + Qwen + 单 SSE 流 + decision 控制` 为核心的 Agent Runtime 骨架。
- 冲突消解一：关于“runtime 真相是否已迁出 local”。最终判断为“执行真相已迁出，规划与策略真相只部分迁出”。依据是 `EinoAIRuntime` 已是默认正式路径，但 planner 仍以 regex 和硬编码模板为主，且部分共享逻辑仍散落在 `internal/service/system/aiRuntimeLocal.go:347`。
- 冲突消解二：关于“下一阶段主线应该是多节点恢复，还是先做 eval/observability”。最终判断是“主线做运行控制面闭环，eval/observability 作为同阶段验收与治理支撑”。原因是多节点 owner 路由和 durable resume 直接决定系统是否能在生产里成立，而 eval/observability 决定它是否可持续维护。
- 冲突消解三：关于“等待确认期间新消息策略”。资料研究更倾向先明确 `reject`，代码现状也是 `busy reject`，而主文档曾写“新消息优先”。最终建议第三阶段先冻结为“显式 `reject while waiting`”，同步修正文档；等 owner 路由和 recovery 做完，再考虑真正的 interrupt 抢占。原因是现在直接支持抢占，会把 partial tool/UI/trace 清理和多节点恢复耦合到一起，风险过高。

# 已完成能力

- `EinoAIRuntime + Qwen` 已成为默认正式运行时骨架。依据见 `internal/core/config.go:58`、`internal/service/system/aiRuntimeFactory.go:16`、`internal/infrastructure/ai/eino/agent_factory.go:42`。
- 共享 planner 已从单纯 local runtime 内部逻辑中抽出，`LocalAIRuntime` 与 `EinoAIRuntime` 已复用同一份规划入口。依据见 `internal/service/system/aiPlanner.go:10`、`internal/service/system/aiRuntimeLocal.go:152`、`internal/service/system/aiRuntimeEino.go:161`。
- 正式上下文已收回服务端推导，前端上传的兼容字段不再是唯一真相。依据见 `internal/service/system/aiContext.go:28`、`internal/service/system/aiContext.go:71`。
- `get_task_snapshot`、`get_progress_snapshot`、`search_project_docs` 已进入统一 Eino 工具链，task/progress 已不再依赖 local 假结果。依据见 `internal/service/system/aiRuntimeEino.go:107`、`internal/infrastructure/ai/eino/task_progress_tools.go:11`、`internal/infrastructure/ai/eino/docs_tool.go:27`。
- 单节点原流 interrupt/resume 已闭合，`checkpoint_id / resume_target_id / tool_name / runtime_name` 已形成最小 runtime state。依据见 `internal/service/system/aiSvc.go:297`、`internal/service/system/aiSvc.go:307`、`internal/service/system/aiRuntimeEino.go:199`、`internal/service/system/aiRuntimeEino.go:297`。
- 外部协议仍然稳定，6 个 API 和 10 个 SSE 事件没有被 Eino/A2UI 改写，且 sink 已具备单调状态合并和 waiting UI 收口能力。依据见 `internal/model/dto/response/aiResp.go:112`、`internal/service/system/aiSink.go:126`、`internal/service/system/aiSink.go:304`。
- 作为面试项目，它已经“可讲”，而且能讲成“业务协议稳定前提下的 Agent Runtime 迁移工程”，不是简单的“接了个模型 SDK”。

# 缺失的关键闭环

- 多节点 interrupt owner 路由未完成。`OwnerNodeID` 已入库，但 `SubmitDecision` 仍直接打当前进程 runtime，没有命令路由。依据见 `internal/service/system/aiSvc.go:313`、`internal/service/system/aiSvc.go:360`。
- durable resume 未完成。checkpoint 已进 Redis，但等待中的 decision 通道仍依赖本进程内存 registry；节点切换或进程重启时闭环会断。依据见 `internal/service/system/aiRuntimeLocal.go:22`、`internal/infrastructure/ai/eino/checkpoint_store.go:13`。
- “等待确认期间新消息”存在文档与代码冲突。主文档写过“新消息轮次优先”，当前实现则直接 `busy reject`。依据见 `docs/AI助手架构设计方案.md:189`、`internal/service/system/aiSvc.go:245`。
- 工具执行链虽然统一了，但规划、审批和策略框架还未正式化。当前仍以 regex planner 和 `search_project_docs` 特判审批为主，且部分共享模板仍留在 local runtime 文件中。依据见 `internal/service/system/aiPlanner.go:49`、`internal/infrastructure/ai/eino/approval_middleware.go:17`、`internal/service/system/aiRuntimeLocal.go:314`。
- 权限、审计、超时、内部错误码、fallback 治理还没有形成正式生产边界。当前 factory 会回退 local runtime，但缺少明确审计和指标。依据见 `internal/service/system/aiRuntimeFactory.go:21`。
- 回归与可观测性还不够硬。现有测试已覆盖部分 sink 和 runtime path，但 `skip / revoke / history reload / owner lost / checkpoint missing / multi-node` 仍缺系统化回归；history reload 也缺直接测试。
- 结论上，当前项目最缺的不是“更多工具”，而是 `OwnerNodeID -> command route -> recovery -> policy -> metrics -> regression` 这条生产闭环。

# 下一阶段最值得做的事项

- 只做一条主线时，最推荐路线是：把 AI 子域从“单节点会话 runtime”升级为“可恢复、可路由、可观测的 runtime control plane”。
- 第一优先级是多节点 owner 路由。没有它，`decision` 命中非 owner 节点时就会失败或误判 unavailable，当前 `OwnerNodeID` 只是字段，不是机制。
- 第二优先级是 durable resume。目标不是新增第二条 SSE 流，而是让服务端运行控制面在 owner 节点失联后仍能安全收口或后台恢复。
- 第三优先级是工具执行边界正式化。task/progress/doc 三类工具应统一补上 `ToolExecutionPolicy`、审计记录、超时包装和内部错误码。
- 第四优先级是 fallback 治理。`LocalAIRuntime` 可以继续保留，但只能作为开发或显式 fallback；生产不能静默退化。
- 第五优先级是 eval / regression / observability。先做事件级与 trace 级回归，再做小样本 eval，不要先陷入最终文本质量比较。
- 如果只允许补 2 到 3 个 feature 来增强面试竞争力，最值钱的是：多节点 owner 路由与 recovery、AI observability + fallback dashboard、事件级 regression harness。

# 面试表达建议

- 当前项目可以讲，但要讲成“业务协议稳定前提下，把本地占位 runtime 迁移为 Eino 正式运行时骨架，并保住 interrupt/resume、checkpoint、single SSE stream、decision control 的工程项目”。
- 当前最核心的 3 个亮点是：一，`EinoAIRuntime + Qwen` 已成为正式骨架而不是 demo fallback；二，顶层协议没有被框架示例绑架，仍保持单 SSE 流和独立 decision 控制口；三，task/progress/doc 已被统一进正式工具链，消息状态和 interrupt 状态已具备单调收口能力。
- 当前最危险的 3 个短板是：一，多节点 owner 路由与 durable resume 未闭合；二，新消息抢占策略未正式定稿且文档与代码不一致；三，缺少生产级审计、fallback 监控和事件级 regression。
- 面试时不要把它包装成“全功能 Agent 平台”，而应准确表述为“第二阶段正式骨架已经完成，第三阶段准备补运行控制面闭环”。这种表述更真实，也更能体现架构判断。
- 一版可直接使用的项目介绍话术：`我做的是一个面向业务对话的 Agent Runtime 迁移项目。核心不是接大模型，而是在不改 6 个 API 和 10 个 SSE 事件协议的前提下，把原来的 LocalAIRuntime 迁到 Eino，并把 interrupt/resume、checkpoint、Qwen 模型接入、任务/进度/文档工具链统一起来。现在项目已经完成第二阶段正式骨架，下一阶段重点不是加更多工具，而是补多节点 owner 路由、durable resume、fallback 治理和事件级 regression，让它真正具备生产级运行控制能力。`
- 如果被追问“为什么不直接用 A2UI 或第二条续跑流”，推荐回答：`因为这个项目首先要守住既有业务协议和历史消息模型。Eino 负责 runtime，业务层继续输出稳定 SSE 事件；decision 只做控制输入，不新开第二条续跑流，这样前端和历史回放成本最低，也更符合现有架构边界。`

# 推荐实施计划

第一批已落地范围：

- `SubmitDecision` 已改成“先持久化，再按 `OwnerNodeID` 路由”，remote decision 不再依赖命中本地 runtime 才算成功。
- Redis envelope / lease / recovery worker 已接入；后台 durable resume 首批只覆盖 `runtime_name=eino` 且 `tool_name=search_project_docs` 的 interrupt。
- “等待确认期间新消息”当前阶段已冻结为 `reject while waiting`，避免在 owner 路由和 recovery 未完全稳定前引入抢占语义。

1. 先做 `AIRuntimeCommandBus`，让 `SubmitDecision` 和 `RevokeUserSessions` 按 `OwnerNodeID` 路由到真正的 owner 节点；本地 `sessionRegistry` 保留，但只负责本节点等待会话唤醒。
2. 再做 owner lease 和 runtime envelope，至少在 Redis 中补齐 `interrupt_id / owner_node_id / checkpoint_id / resume_target_id / tool_name / lease_expire_at`，并引入 recovery worker。
3. 将 durable resume 定义为“服务端运行控制面可恢复”，不是“客户端断线自动续流”；第一版 recovery 只要求两件事：owner 丢失但未收到 decision 时安全收口，owner 丢失但已收到 decision 且 checkpoint 完整时可后台恢复到终态。
4. 补 `ToolExecutionPolicy`、AI 审计表、统一 timeout wrapper 和内部 machine error code；外部 HTTP/SSE 协议不变，内部治理能力增强。
5. 为 runtime factory 增加 `ai.allow_local_fallback` 及配套审计、指标、结构化日志；生产默认禁用静默 fallback。
6. 建最小 regression harness，先验 plan、tool 选择、事件序列、interrupt 状态，而不是先比最终文本；第一批样例至少覆盖 `lightweight / task / progress / doc confirm / doc skip / cancel while waiting / revoke while waiting / history reload / owner lost / checkpoint missing`。
7. 建 AI observability 最小指标：plan latency、tool latency、interrupt wait duration、decision-to-resume duration、resume success rate、recovery success rate、fallback rate、checkpoint miss rate。
8. 同步修正文档，把“等待期间新消息”策略在第三阶段先固定为 `reject while waiting`，并说明这是当前阶段的收敛策略，不是永久产品结论。

# 风险与注意事项

- 不要在第三阶段引入第二条 SSE 续跑流，也不要让前端直接依赖 runtime 原始事件；继续保持业务 SSE 事件作为稳定协议面。
- 不要把 interrupt 继续当普通错误处理。公开资料和当前代码方向都支持把它当一等控制语义；错误、超时、cancel、interrupt 应分开建模。
- 不要在 owner 路由和 recovery 未完成前就实现“等待期间新消息抢占”；否则 partial tool 输出、waiting UI、trace 清理会和多节点恢复纠缠在一起。
- 不要让 Qwen 继续以“假装 OpenAI provider”的语义存在；正式路径应继续是 Eino 原生 `qwen` provider + DashScope compatible endpoint。
- 需要尽早冻结 checkpoint 相关序列化和 identity key 语义，至少包括 `checkpoint_id / interrupt_id / resume_target_id / owner_node_id / runtime_name`；否则后续线上恢复会被兼容性拖垮。
- 文档要与代码同步，尤其是“新消息策略”“fallback 语义”“durable resume 边界”三处；这三处如果继续口径不一致，会同时影响开发、测试和面试叙述。
- 资料依据主要来自官方和主流可靠资料：Eino Runner/HITL/interrupt-resume 重构、Eino Qwen 组件、LangGraph durable execution/HITL、LangSmith double-texting 与 eval 指南、OpenAI agent eval/trace grading 指南。下一阶段的设计应继续优先对齐这些成熟机制，而不是自造第二套语义。
