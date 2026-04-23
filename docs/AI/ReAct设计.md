建议按“生产级 Agent Runtime”来重构，而不是在现有 `Stream()` 里手写 ReAct 循环。成熟路线是：**用 Eino ADK `ChatModelAgent + Runner` 承担 ReAct 编排，你的项目继续保留 `AIService -> Runtime -> Sink -> Projector -> Repository` 这条业务边界。**

**目标架构**
```text
Controller
  -> AIService
    -> AgentRuntime(domain/ai.Runtime)
      -> Eino ADK Runner
        -> ChatModelAgent(ReAct)
        -> ToolsNode
      -> EventAdapter
    -> aiStreamSink
      -> SSE
      -> aiMessageProjector
      -> ai_messages.trace_items_json / ui_blocks_json / scope_json
```

ReAct 内部流程是：

```text
Reason: 模型判断是否需要工具
Action: 生成 tool_call
Act: 执行业务 Tool
Observation: tool_result 回灌给模型
Repeat: 直到模型不再调用工具，输出最终回答
```

**推荐编码顺序**

1. **先扩展事件协议**
   当前 [event.go](<D:\workspace_go\test\go\personal_assistant\internal\domain\ai\event.go:8>) 只有 token/done/error，不够表达 ReAct。建议新增：
   - `agent_step_started`
   - `tool_call_started`
   - `tool_call_finished`
   - `tool_call_failed`
   - `tool_call_waiting_confirmation`
   - `structured_block`
   - `thinking_summary`

   注意：不要展示模型原始思维链。成熟产品通常展示“执行摘要 / 工具轨迹 / 可审计步骤”，不是裸 Chain-of-Thought。

2. **恢复 `aiProjector` 的 trace 投影能力**
   你现在的 [aiProjector.go](<D:\workspace_go\test\go\personal_assistant\internal\service\system\aiProjector.go:14>) 明确只处理基础文本。下一步应让它重新维护：
   - `TraceItemsJSON`：工具调用轨迹、耗时、状态、确认动作
   - `UIBlocksJSON`：结构化结果卡片
   - `ScopeJSON`：当前用户、组织、业务上下文

   好处是前端刷新历史消息时，仍能看到 Agent 做过什么。

3. **新增 Tool 业务边界**
   不建议让 Eino Tool 直接查 DB。建议这样拆：

   ```text
   internal/service/system/aiToolService.go
      负责业务查询、权限、组织范围、结果裁剪
   
   internal/infrastructure/ai/eino/tools/*.go
      只做 Eino tool schema + adapter
   ```

   工具参数里不要相信模型传来的 `user_id/org_id`，这些必须从 `StreamInput.UserID` 和服务端上下文注入。

4. **第一批 Tool 只做只读能力**
   建议从低风险工具开始：
   - `get_current_user_scope`
   - `get_oj_daily_stats`
   - `search_project_docs`
   - `get_conversation_summary`

   先不要做写操作。写操作后面必须接 interrupt/confirmation。

5. **把 Eino runtime 从 ChatModel 改为 AgentRuntime**
   当前 [runtime.go](<D:\workspace_go\test\go\personal_assistant\internal\infrastructure\ai\eino\runtime.go:90>) 是直接 `r.model.Stream(...)`。重构后这里不再直接调 ChatModel，而是：
   - 构造 `ChatModelAgent`
   - 注册工具 `ToolsConfig`
   - 用 `Runner` 执行
   - 把 `AgentEvent` 转成你的 `aidomain.Event`
   - 继续通过 `sink.Emit()` 输出

   Eino 官方的 `ChatModelAgent` 内部就是 ReAct；如果只是简单 ReAct，也可以用 `react.NewAgent`，但你后续要 interrupt/resume/checkpoint，所以更建议 ADK Runner。

6. **加 Planner/白名单，不要把所有工具暴露给模型**
   成熟 Agent 系统不会每轮都给模型全量工具。建议在 `AIService` 调 runtime 前先做一个轻量 plan：

   ```text
   用户问题 -> intent/plan -> allowed_tools -> AgentRuntime
   ```

   例如普通闲聊不给工具；问 OJ 数据才开放 OJ 工具；问项目文档才开放文档检索。

7. **第二阶段再做 interrupt/resume**
   你的 [AIInterrupt](<D:\workspace_go\test\go\personal_assistant\internal\model\entity\ai.go:65>) 表已经有基础字段，可以后续接：
   - 高风险工具调用前暂停
   - 写入 `AIInterrupt`
   - SSE 发 `tool_call_waiting_confirmation`
   - 用户确认后 `Runner.Resume(checkpoint_id)`
   - projector 更新 trace 状态

   写操作、跨组织查询、发送通知、长期记忆写入，都应该走这套。

**成熟性检查清单**

- Tool 有超时、限流、最大返回长度。
- Agent 有 `MaxIterations/MaxStep`，避免无限循环。
- Tool 参数和返回值都做 schema 校验。
- Tool 结果做脱敏和摘要，不把大对象直接塞回模型。
- trace 里记录 tool name、status、duration、redacted args、summary。
- 所有 Tool 使用 `context.Context`，权限基于服务端用户上下文。
- 测试覆盖：纯聊天、一次工具调用、多次工具调用、工具失败、超步数、SSE 中断、DB trace 落库。

官方参考我建议看这几份：CloudWeGo [ReAct Agent Manual](https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/)、[Eino ADK ChatModelAgent](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_implementation/chat_model/)、[Agent Runner and Extension](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_extension/)。当前项目要做“市面成熟”的 ReAct，优先走 ADK Runner 这条线。