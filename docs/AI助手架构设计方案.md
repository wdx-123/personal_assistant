# AI 助手架构设计方案

## 1. 文档定位

本文档是 `personal_assistant` AI 子域的唯一正式方案文档，用于统一以下内容：

1. 业务定位与分层边界。
2. Go + Eino 的运行时基线。
3. 前端真实协议、SSE 事件和消息模型。
4. AI 回复区的四类可见内容规则。
5. OpenAPI / Apifox 对外契约。
6. V1 验收标准。

旧文档 [AI助手后端对接-go-eino-V1.md](./AI助手后端对接-go-eino-V1.md) 只保留迁移说明，不再维护独立结论。

项目级 SSE 连接层、回放层、跨节点分发、安全与运维基线，统一以 [SSE实时推送基础设施重构指导文档.md](./SSE实时推送基础设施重构指导文档.md) 为准；本文档只保留 AI 子域协议、运行时和验收结论。

## 2. 真相源与案例基线

本方案固定以下 5 类真相源：

1. `z_cur/UI/src/types/assistant.types.ts`
2. `z_cur/UI/src/stores/assistant.ts`
3. `z_cur/UI/src/components/business/Assistant/**`
4. `z_cur/Eino/eino-examples/quickstart/chatwitheino/docs/ch07_interrupt_resume.md`
5. `z_cur/Eino/eino-examples/quickstart/chatwitheino/docs/ch10_a2ui.md`

结论解释：

1. 前端真实协议和消息模型，以 `z_cur/UI` 现有实现为准。
2. Eino 运行时基线，以 `Interrupt / Resume + Checkpoint` 案例为准。
3. A2UI 只借鉴适合声明式渲染的部分，不直接照搬 `ch10` 的顶层协议。

## 3. 设计结论

### 3.1 项目定位

AI 助手是 `personal_assistant` 的正式业务子域，不独立拆仓，不做脱离业务上下文的 demo。

V1 固定提供 4 类能力：

1. 我的任务汇报。
2. 指定范围任务汇总。
3. 用户进度分析。
4. 正式项目文档问答。

### 3.2 Eino 运行时结论

V1 从首版开始就把 `Interrupt / Checkpoint` 作为必选能力，不采用“业务 Service 自己维护待确认状态，再发起第二条 SSE 流”的旧方案。

固定运行时如下：

1. `ChatModelAgent` 负责模型与 Tool 调度。
2. `Runner` 负责执行与恢复。
3. `Approval / Interrupt` 负责人工确认节点。
4. `CheckPointStore` 负责运行时恢复点。
5. 业务 Service 负责权限、会话、消息、SSE 事件映射与持久化收口。

### 3.3 协议结论

前端协议采用“业务事件流 + 内嵌 A2UI block”的混合模型，不切换到纯 A2UI 顶层协议。

固定原则：

1. 顶层 SSE 仍是业务事件协议。
2. `A2UI` 只作为 `structured_block.ui_block` 的渲染载荷出现。
3. `trace_items` 与 `scope` 继续是恢复与上下文真相字段。
4. `content` 是唯一正式最终答案正文。
5. `ui_blocks` 只承载“思考摘要 / 工具意图 / 等待用户”三类可见结构化块。

### 3.4 单流结论

V1 固定为“单条聊天 SSE 流 + 独立控制接口”：

1. `POST /ai/conversations/{id}/stream`
   开启唯一聊天事件流。
2. `POST /ai/conversations/{id}/interrupts/{interrupt_id}/decision`
   只提交确认决策，不返回第二条 SSE 流。

运行语义：

1. 命中 interrupt 后，`stream` 不结束业务轮次，只进入等待确认阶段。
2. 服务端持续保活同一条 SSE 连接。
3. 前端调用 decision 接口后，服务端在原流内 `Resume` 并继续输出后续事件。
4. V1 不承诺“中途断流后重新附着到同一轮运行”；断流即本轮失败或停止。

## 4. 总体架构

### 4.1 后端分层

后端继续沿用当前仓库分层：

1. `controller`
   负责 HTTP 绑定、SSE 写出、上下文提取、错误返回。
2. `service`
   负责会话编排、Agent 调用、权限收口、协议映射、落库。
3. `repository`
   负责会话、消息、审计、待确认记录的持久化。
4. `infrastructure`
   负责模型适配器、Eino 初始化、CheckpointStore、文档加载器。
5. `router`
   负责把 AI 路由挂入登录态业务分组。

### 4.2 数据真相源

V1 固定两类存储：

1. MySQL
   会话、消息、审计记录的业务真相源。
2. Redis
   `CheckPointStore` 与运行时 interrupt / resume 所需的状态存储。

Redis 不是业务消息真相源；消息历史仍以 MySQL 为准。

### 4.3 权限边界

权限必须先于模型回答，而不是靠提示词兜底。

固定规则：

1. 默认范围是“当前登录用户 + 当前组织”。
2. 查询他人、跨组织、管理视角数据时，必须先走现有资源级鉴权。
3. 文档问答只允许读取正式白名单文档。
4. 越权请求在 Service 层直接拒绝，不把敏感数据交给模型。

## 5. AI 回复协议

### 5.1 四类可见内容

单条 assistant 消息只允许出现以下四类可见内容：

1. 思考摘要
   用于展示阶段性判断、当前动作、等待原因和下一步，不泄露原始长推理。
2. 工具意图
   用于说明为什么要调用工具、调用后会得到什么、是否需要确认。
3. 等待用户
   用于明确当前轮次为什么暂停、需要用户确认什么、确认后会发生什么。
4. 最终正文
   用于承载正式回答，是唯一正式结果正文。

### 5.2 消息模型

一条 assistant 消息不是单纯字符串，而是以下组合：

1. `content`
   Markdown 最终正文；`assistant_token` 和 `message_completed.content` 只承载它。
2. `trace_items`
   工具执行记录与恢复真相。
3. `ui_blocks`
   结构化可见块，只允许：
   - `thinking_summary_block`
   - `tool_intent_block`
   - `waiting_user_block`
4. `scope`
   可选上下文元数据；仅在复杂范围、跨用户、跨组织或带文档白名单时返回，但默认不单独展示。
5. `status / error_text`
   消息过程态与错误态。

历史消息返回时，后端应优先补齐 `ui_blocks`，并保留 `trace_items / scope` 以支持状态恢复。

### 5.3 渲染规则

前端固定按以下逻辑渲染单条 assistant 消息：

1. 思考摘要
2. 工具意图
3. 等待用户
4. 最终正文

补充规则：

1. 问候语、寒暄、感谢和无业务目标的短消息，只展示最终正文。
2. 工具执行记录只作为 `工具意图` 内的折叠执行记录存在，不再作为独立可见模块。
3. 等待用户块只负责说明暂停点；真正可点击的确认按钮仍固定放在消息列表下方、输入框上方的独立操作条。
4. 用户在等待期间输入新消息时，以新消息轮次为最高优先级；旧等待态保留为历史，但不再继续假设后续结果。

### 5.4 SSE 事件

顶层事件固定保留以下 10 个：

1. `conversation_started`
2. `assistant_token`
3. `tool_call_started`
4. `tool_call_finished`
5. `tool_call_waiting_confirmation`
6. `tool_call_confirmation_result`
7. `structured_block`
8. `message_completed`
9. `error`
10. `done`

其中：

1. `tool_call_waiting_confirmation` payload 必须带 `interrupt_id`。
2. `structured_block` 允许承载 `scope / ui_block`。
3. `tool_call_confirmation_result` 表示“决策已受理并已恢复或跳过”，而不是第二条流的起点。
4. `assistant_token` 只能用于流式输出最终正文，不能混入结论壳、指标壳或等待提示。

## 6. 局部 A2UI 设计

### 6.1 适用边界

| 分类 | 是否采用 A2UI | 结论 |
| --- | --- | --- |
| 思考摘要 | 是 | 适合声明式展示阶段性判断 |
| 工具意图 | 是 | 适合声明式展示目的、必要性、收益与确认要求 |
| 等待用户 | 是 | 适合声明式展示暂停原因和下一步 |
| 最终正文 | 否 | 继续使用 Markdown 作为唯一正式结果正文 |
| 会话列表 | 否 | 属于业务页面壳层 |
| 主布局 | 否 | 属于页面壳层，不应协议化 |
| 权限判断 | 否 | 属于业务规则，不应协议化 |
| 工具确认状态机 | 否 | 属于业务状态机与 interrupt 运行时 |
| SSE 协议 | 否 | 继续采用业务事件协议 |
| 消息持久化 | 否 | 继续以业务字段为真相 |

### 6.2 A2UI 子集

基础布局组件固定为：

1. `Text`
2. `Row`
3. `Column`
4. `Card`

业务扩展组件固定为：

1. `Badge`
2. `BulletList`

Block 类型固定为：

1. `thinking_summary_block`
2. `tool_intent_block`
3. `waiting_user_block`

### 6.3 责任边界

必须固定以下责任边界：

1. `trace_items` 负责工具执行记录与恢复真相。
2. `tool_intent_block` 只负责解释为什么需要工具，以及这轮工具调用的意图和收益。
3. `waiting_user_block` 只负责表达暂停原因和等待点。
4. `thinking_summary_block` 只承载“当前判断 / 当前动作 / 等待原因 / 下一步”的摘要，不是原始模型推理全文。
5. `scope` 是元数据，不是默认可见内容。

## 7. 后端实现方案

### 7.1 路由

AI 接口固定为 6 个：

1. `POST /ai/conversations`
2. `GET /ai/conversations`
3. `GET /ai/conversations/{id}/messages`
4. `DELETE /ai/conversations/{id}`
5. `POST /ai/conversations/{id}/stream`
6. `POST /ai/conversations/{id}/interrupts/{interrupt_id}/decision`

### 7.2 Service 责任

AI Service 必须承担以下职责：

1. 会话 CRUD 与消息历史装载。
2. 用户 / 组织 /任务 / 文档范围裁剪。
3. Tool 选择与参数装配。
4. Eino Agent / Runner 调用。
5. Interrupt 命中后的运行时恢复。
6. SSE 事件映射。
7. 会话、消息与审计落库。

另外固定一条产品侧门控规则：

1. 问候语、寒暄、感谢和无业务目标的短消息，直接走轻量直答路径。
2. 只有进入任务分析、进度分析、范围汇总、文档问答这类业务意图时，才进入重型工具链路。
3. `scope` 默认不前台展示；只有确实存在复杂范围时才回传。

### 7.3 Tool 约束

V1 固定 4 类 Tool：

1. `get_my_task_report`
2. `get_scoped_task_report`
3. `get_user_progress_insight`
4. `search_project_docs`

约束固定如下：

1. Tool 不直接散落 SQL。
2. Tool 统一通过现有 Service 或只读 Facade 获取数据。
3. 文档类 Tool 只能访问正式白名单文档。
4. Tool 输出必须能映射成 `trace_items`，并驱动 `tool_intent_block / waiting_user_block / 最终正文`。

## 8. OpenAPI / Apifox 结论

Apifox 契约以两份文件同步维护：

1. `z_cur/UI/docs/apifox/ui-module.openapi.json`
2. `go/personal_assistant/docs/apifox/ai_assistant.openapi.json`

两者必须保持一致，固定约束如下：

1. CRUD 与 decision 接口走 JSON BizResponse。
2. `stream` 接口只输出 `text/event-stream`。
3. 不再出现旧的工具续跑流接口描述。
4. `thinking_summary_block / tool_intent_block / waiting_user_block / interrupt_id` 都必须进入 schema。
5. SSE 示例必须体现“原流等待 + decision 接口控制 + 原流继续输出”。
6. 文档必须明确：`content` 是唯一正式最终答案正文；`trace_items / scope` 是恢复与上下文字段，不是默认可见内容。

## 9. V1 验收标准

### 9.1 协议验收

1. 主文档已完全切到“四类可见内容”心智。
2. `tool_call_waiting_confirmation` 与 `tool_call_confirmation_result` 都带 `interrupt_id`。
3. 历史消息能重建 `content / trace_items / ui_blocks / scope`。
4. 不再出现任务卡、进度卡、文档卡和独立工具轨迹块。

### 9.2 前端验收

1. `structured_block.ui_block` 只渲染 3 类 block。
2. 问候语与寒暄类短消息走轻量直答路径，只出现最终正文。
3. 复杂问题先出现思考摘要，再按需要出现工具意图与等待用户。
4. 工具执行记录默认折叠在工具意图内部，不再单独占区。
5. 等待确认时，真正交互入口只保留底部独立确认条。
6. 用户在等待期间输入新消息时，新消息轮次优先，旧轮次停止等待。
7. decision 提交后不再开启第二条流，原流继续完成。

### 9.3 后端验收

1. 首版即采用 `Interrupt / Resume + Checkpoint`。
2. `stream` 事件顺序正确，keepalive 不影响前端解析。
3. decision 接口只提交控制命令，不直接返回 SSE。
4. `interrupt_id` 与会话归属、权限、当前运行实例关系校验正确。

## 10. 后续扩展边界

V2 可以考虑：

1. 更完整的知识库索引与检索。
2. 更复杂的 Workflow / Multi-Agent。
3. 断流后附着到同一轮运行的恢复能力。
4. 更通用的 A2UI 协议抽象。

这些都建立在 V1 四类可见内容、单流恢复和业务闭环稳定之后，不提前抢跑。
