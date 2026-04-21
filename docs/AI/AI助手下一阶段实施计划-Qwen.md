# AI Runtime -> Eino 第二阶段实施计划（Qwen 版）

## 1. 文档定位

本文档是第二阶段的实施说明，补充主文档 [AI助手架构设计方案.md](./AI助手架构设计方案.md) 的落地范围、默认配置和验收口径。

第二阶段只做三件事：

1. 正式模型路径默认切到 `Qwen + DashScope compatible-mode`。
2. 把 task / progress / doc 三类正式能力统一收进 `EinoAIRuntime`。
3. 把上下文真相从前端兼容字段收回服务端。

## 2. 当前正式结论

1. `AIRuntime` 继续保留为 service seam，`AIService` 不直接依赖具体模型 SDK。
2. `LocalAIRuntime` 继续保留，但只作为 mock / test / fallback。
3. 6 个 HTTP API、10 个 SSE 事件、单流 `decision` JSON 控制接口保持不变。
4. 正式 provider 默认值改为：
   - `ai.provider=qwen`
   - `ai.base_url=https://dashscope.aliyuncs.com/compatible-mode/v1`
   - `ai.model=qwen-plus`
5. `ContextUserName / ContextOrgName` 保留为兼容字段，但不再作为正式可信输入。

## 3. 实现范围

### 3.1 模型与运行时

1. 模型工厂新增 `qwen` provider，正式实现使用 `github.com/cloudwego/eino-ext/components/model/qwen`。
2. `EinoAIRuntime` 对非 lightweight 请求统一走 Eino runner。
3. `AIRuntimePlan` 继续保留，但 plan 逻辑从 `LocalAIRuntime` 中抽出，作为共享 planner。

### 3.2 工具执行

第二阶段固定 3 个工具：

1. `get_task_snapshot`
   - 无需确认。
   - 读取当前用户可见任务的最新执行快照。
2. `get_progress_snapshot`
   - 无需确认。
   - 读取最近 7 天训练进度与当前 OJ 分数。
3. `search_project_docs`
   - 需要确认。
   - 继续走 `interrupt / resume / checkpoint`。

这些工具都由 Eino 执行，但外部仍继续消费现有业务 SSE 事件映射。

### 3.3 上下文与持久化

1. scope 中的人名、组织名由服务端推导。
2. `RuntimeStateJSON` 固定保留以下字段：
   - `runtime_name`
   - `checkpoint_id`
   - `resume_target_id`
   - `tool_name`
3. 历史消息恢复仍以 MySQL 中的消息快照和 interrupt 终态为准。

## 4. 验收口径

1. 非 lightweight 请求不再正式回退到 local execute 分支。
2. task / progress 工具通过 Eino 工具链触发，并继续映射为已有 `tool_call_started / tool_call_finished` 事件。
3. `search_project_docs` 的 confirm / skip 继续在原 SSE 流内恢复，不新增第二条 SSE。
4. 兼容字段 `ContextUserName / ContextOrgName` 缺失或伪造时，scope 仍以服务端真相为准。
5. `LocalAIRuntime` 只用于 fallback / test，不作为正式默认运行时。

## 5. 配置建议

开发环境建议：

1. 默认模型：`qwen-plus`
2. 成本敏感场景可改为：`qwen3.5-flash`
3. 若 `ai.api_key`、`ai.model` 或 Redis checkpoint 条件不满足，运行时允许回退 `LocalAIRuntime`，但应视为非正式模式。
