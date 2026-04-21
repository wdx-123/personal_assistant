# AI 助手后端对接说明（已并入主文档）

本文档不再作为独立方案维护，正式内容已并入：

- [AI助手架构设计方案.md](./AI助手架构设计方案.md)

当前迁移结论固定如下：

1. V1 从首版开始采用 go-eino `Interrupt / Resume + Checkpoint`，不再使用“业务 Service 管理待确认状态 + 第二条 SSE 流”的旧方案。
2. 顶层前端协议继续采用业务 SSE 事件流，不切换到纯 A2UI 协议。
3. AI 回复区的正式可见内容只保留四类：
   - 思考摘要
   - 工具意图
   - 等待用户
   - 最终正文
4. `最终正文` 以 Markdown 为唯一正式答案载体；任务卡、进度卡、文档卡不再作为正式用户可见协议。
5. 工具执行记录继续保留在 `trace_items` 中，但只作为 `工具意图` 内的折叠记录，不再作为独立可见模块。
6. `scope` 只作为可选上下文元数据保留，普通同上下文对话默认不展示。
7. 正式接口固定为：
   - `POST /ai/conversations`
   - `GET /ai/conversations`
   - `GET /ai/conversations/{id}/messages`
   - `DELETE /ai/conversations/{id}`
   - `POST /ai/conversations/{id}/stream`
   - `POST /ai/conversations/{id}/interrupts/{interrupt_id}/decision`
8. 旧的工具续跑流接口全量废弃。
9. 第一阶段实现继续保留 `AIRuntime` 作为业务缝；正式运行时默认走 `EinoAIRuntime`，`LocalAIRuntime` 只保留为 mock / test / fallback。
10. 第二阶段正式模型路径默认切到 `Qwen + DashScope compatible-mode`，不再默认走 `OpenAI / Ark`。
11. 第二阶段开始，`task / progress / doc` 三类正式能力统一由 Eino 工具执行；前端上传的 `ContextUserName / ContextOrgName` 只保留兼容，不再作为正式真相输入。

若后续需要补充实现细节、OpenAPI 变更或前后端联调约束，只更新主文档，不再回写本文件。
