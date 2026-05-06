# 目标

把记忆治理升级到 `LLM 提议 ttl_hint/confidence + Policy 裁决 expires_at` 的形态，避免靠正则从原文硬抠 TTL。

# 范围

- AI 记忆写回候选结构。
- LLM extractor 输出 schema 与解析。
- `aiMemoryPolicy.ResolveTTL` 的入参和裁决逻辑。
- facts/documents 写回时的 TTL 使用。
- 对应单元测试与文档说明。

# 改动

- 在 `internal/domain/ai` 增加 `MemoryTTLHint`，支持 `default / persistent / duration / until_date / session_only` 这类受控类型。
- 在 `MemoryFactCandidate` 和 `MemoryDocumentCandidate` 增加 `Confidence`、`TTLHint` 字段。
- LLM extractor prompt 要求输出结构化 `ttl_hint`，并把模型输出转成候选 hint；不允许 LLM 直接决定最终 `expires_at`。
- Policy 新增或重构 `ResolveTTL(namespace, memoryType, hint)`：
  - `user_preference` 默认长期，拒绝短期 duration 覆盖。
  - `oj_goal` 默认 30 天，允许合法 duration hint，并对天数做范围约束。
  - `oj_profile` 默认 60 天。
  - `org_learning_pattern` 默认 14 天。
  - 非法、越界、解析失败或不匹配 namespace 的 hint 回退 policy 默认 TTL。
- 写回链路使用 policy 裁决后的 `ExpiresAt`，仍由 Service 构造最终 entity。
- 文档补充：TTL 不是正则硬抠，而是 LLM 提议类型化时间语义，policy 做 allowlist、clamp 和最终计算。

# 验证

- `go test ./internal/infrastructure/ai/memory`
- `go test ./internal/service/system -run "AIMemoryWriteback|AIMemoryPolicy"`
- `go test ./internal/service/system`
- `git diff --check`

# 风险

- 当前工作区已有 AI 记忆/混合召回未提交改动，实施时只做增量修改，不回滚已有改动。
- 不扩展 org/platform_ops 写回权限，避免和授权链路混在本次变更里。
- 不改 DB schema；`ttl_hint` 只参与入库前裁决，最终仍只写现有 `expires_at`。

# 执行顺序

1. 用户确认计划后，将本文件改名为 `approved-memory-ttl-hint-policy.md`。
2. 扩展 domain candidate 和 TTL hint 类型。
3. 更新 LLM extractor prompt、解析和候选构造。
4. 重构 policy TTL 裁决逻辑与写回调用点。
5. 补测试、更新文档、运行验证。

# 待确认

请确认是否按本计划实施。
