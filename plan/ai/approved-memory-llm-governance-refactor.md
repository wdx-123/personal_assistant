# 目标

在 AI 记忆写回链路中引入真实 LLM extractor，并保持 `LLM 提议 + Policy 裁决` 的治理边界。

# 范围

- 更新 `docs/AI/设计流程.md` 中 `### 2. 记忆治理` 的设计描述。
- 增加 memory extractor 配置项，默认仍走规则抽取。
- 在 `internal/infrastructure/ai/memory` 增加 LLM extractor。
- 重构 `AIMemoryService` 的 extractor 装配，LLM 失败时回退规则抽取。

# 改动

- `config.AIMemory` 新增 `extractor_mode`、`extract_timeout_seconds`、`extract_max_chars`。
- `AI_MEMORY_EXTRACTOR_MODE=llm` 时启用真实 LLM 候选提议。
- LLM 输出只转成 `Fact / Document / ConversationSummary` 候选，不决定最终 scope、visibility、TTL、dedup 或落库。
- 第一版只允许 `self` 个人记忆候选。

# 验证

- `go test ./internal/infrastructure/ai/memory`
- `go test ./internal/service/system -run "AIMemoryWriteback|AIMemoryPolicy"`
- `go test ./internal/core -run TestInitConfigBindsAIMemoryAndQdrantCompatibility`

# 风险

- 当前工作区已有 AI 记忆/混合召回未提交改动，本次只做增量修改，不回滚已有改动。
- LLM JSON 输出不稳定，因此需要严格解析、校验和 fallback。

# 执行顺序

1. 计划流转为 approved。
2. 增加配置字段、默认值和环境变量绑定。
3. 实现 LLM extractor 与 fallback。
4. 更新 Service 装配和文档。
5. 补充测试并运行目标测试。

# 待确认

用户已通过 `PLEASE IMPLEMENT THIS PLAN` 明确确认执行。
