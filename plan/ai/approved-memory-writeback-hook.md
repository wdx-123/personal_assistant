# 目标

实现 AI 记忆第 3 步 `memory writeback hook`：在一轮流式对话成功完成后，按配置触发记忆写回，将可保留的 summary、facts、documents 经过治理规则后写入现有记忆表。

# 范围

- 只实现写侧生产链路，不接第 4 步上下文恢复。
- v1 使用规则抽取器和稳定接口，不使用 LLM 做结构化抽取。
- 自动写入范围保守限定为个人 self memory；org/platform 共享记忆后续在授权能力收口后再开启。
- 不新增数据库表，复用 `ai_memory_facts`、`ai_memory_documents`、`ai_conversation_summaries`。

# 改动

- 扩展 `domain/ai` 写回协议，定义抽取输入、抽取结果、summary draft 与 extractor 接口。
- 新增 `internal/infrastructure/ai/memory` 规则抽取器，生成保守的 summary、明确表达的个人 fact，以及知识型长回答 document。
- 改造 `AIMemoryService.OnTurnCompleted`，编排读取消息快照、调用 extractor、走 `aiMemoryPolicy` 治理、写入 Repository。
- 在 AI 流式成功收尾后触发 writeback；配置关闭时 no-op，异步模式下后台执行并记录失败日志。
- 补充必要 repository 查询能力，用于 fact 覆盖判断和消息快照读取。

# 验证

- 新增/更新 writeback、规则抽取器、policy 集成相关单测。
- 运行 `go test ./internal/service/system ./internal/repository/system ./internal/domain/ai ./internal/infrastructure/ai/memory`。

# 风险

- 规则抽取器较保守，v1 宁可少写，不自动扩大共享记忆范围。
- writeback 是辅助链路，失败不应影响用户本轮回答。
- document 只先落 MySQL，不做 embedding/chunk/Qdrant。

# 执行顺序

1. 将本计划从 pending 流转为 approved。
2. 扩展 domain 协议与 extractor 接口。
3. 实现规则抽取器。
4. 补 repository 查询能力。
5. 实现 `AIMemoryService.OnTurnCompleted` 和 AIService hook。
6. 补测试并运行验证。

# 待确认

用户已明确要求 “PLEASE IMPLEMENT THIS PLAN”，按该确认执行。
