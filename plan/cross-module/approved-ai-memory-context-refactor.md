# 目标

一次性重构当前 AI 记忆链路，把上下文装配升级为：`latest summary head + filtered facts + bounded RAG + token-budget recent turns`，并把大工具输出从“原样回喂模型”改成“预算受控回喂”。

# 范围

- `internal/service/system`
- `internal/infrastructure/ai/memory`
- `internal/infrastructure/ai/eino`
- `internal/model/config`
- 少量 `internal/domain/ai`、`internal/repository/*`

# 改动

- recent turns 从固定轮数升级为 token budget。
- summary 改成“最新决策优先 + LLM 优先回退 + 按窗口 full refresh/head update”。
- facts 增加 query 相关的 namespace 过滤和重要性排序。
- 工具结果只在模型回喂层做主动压缩，trace 展示保持不变。
- RAG 继续纳入混合上下文，但维持当前 self-scope 和 fail-open 语义。

# 验证

- 补齐 `aiMemoryRecall_test.go`
- 补齐 `aiMemoryWriteback_test.go`
- 补齐 `runtime_tools_test.go`
- 补齐 `aiContext_test.go`
- 运行最小必要 Go 测试

# 风险

- 当前工作区已有相关 AI memory 改动，实施时必须避免覆盖现有未提交内容。
- summary 刷新策略、facts 过滤、tool output 压缩会同时影响上下文质量与 token 成本，需要靠测试和 diagnostics 保底。

# 执行顺序

1. 配置和 domain/repository 接口扩展
2. recent turns token budget 与 diagnostics
3. summary writeback/refresh 策略重构
4. facts 过滤与排序
5. tool output 主动压缩
6. RAG 拼装预算与顺序调整
7. 测试补齐与回归验证

# 待确认

已确认并进入实施：

- summary：LLM 优先回退
- summary full refresh 频率：按 `SummaryRefreshEveryTurns`
- facts：查询相关优先
- 工具压缩：只压模型回喂层
