# 目标

在已冻结的 memory 骨架上实现“记忆治理”第一版规则收口，补齐 `aiMemoryPolicy` 的准入、权限、覆盖、TTL、去重与原因码返回，并同步收紧 `ConversationSummary` 读取契约与 `AIMemoryDocument` 去重字段，保证后续 writeback / recall 都有稳定护栏。

# 范围

- `internal/service/system`
  - 实现 `aiMemoryPolicy` 的输入上下文、决策结果、核心规则函数与配套测试。
  - 视需要补充 `AIMemoryService` 的最小治理接线，但不直接把 memory 接入主链路 writeback。
- `internal/domain/ai`
  - 补充治理所需的稳定类型、来源优先级、scope/visibility 决策辅助结构。
- `internal/model/entity`
  - 为 `AIMemoryDocument` 增加 `content_hash / summary_hash / dedup_key`。
  - 视读取契约需要补充 summary 查询入参结构。
- `internal/repository/interfaces`
  - 收紧 `ConversationSummary` 读取接口，改为按 `conversation_id + user_id + org_id + scope_key` 读取。
- `internal/repository/system`
  - 实现新的 summary 查询条件。
  - 补充 document 去重字段的持久化更新。
  - 增加对应 repository 回归测试。

# 改动

- `aiMemoryPolicy`
  - 新增显式授权上下文，固定只消费：
    - `Principal`
    - `ApprovedOrgScopeKeys` / `ApprovedOrgIDs`
    - `AllowPlatformOps`
  - 新增统一决策返回：
    - `Allowed`
    - `ReasonCode`
    - `Reason`
  - 实现并测试以下规则函数：
    - `ShouldStoreFact`
    - `ShouldStoreDocument`
    - `ResolveScope`
    - `ResolveVisibility`
    - `ResolveTTL`
    - `ShouldOverrideFact`
    - `CanReadMemory`
    - `CanWriteMemory`
- 权限与失败语义
  - `org` scope 只能消费显式传入的已授权组织集合。
  - 禁止通过 `CurrentOrgID` 或 policy 内部调用授权服务推断组织权限。
  - `platform_ops` 必须依赖显式超管事实。
  - 权限依赖缺失一律 `fail closed`。
- 覆盖与优先级
  - `self/user_preference` 采用 `explicit_user_statement > admin_set > tool_verified_summary > model_inferred`。
  - `org/platform_ops` 公共记忆采用 `admin_set > explicit_user_statement > tool_verified_summary > model_inferred`。
  - 其余 `self` namespace 默认沿用私有记忆优先级。
- 去重与 TTL
  - 为 `AIMemoryDocument` 预留 `content_hash / summary_hash / dedup_key`。
  - 第一版 document 去重只做稳定规则去重，不做近似语义合并。
  - 按设计补 namespace / memory type 的 TTL 解析规则。
- Summary 契约收紧
  - `GetConversationSummary` 改为显式匹配：
    - `conversation_id`
    - `user_id`
    - `org_id`
    - `scope_key`
  - 任一不匹配都按“无可用摘要”处理，不允许先按主键命中再信任记录。

# 验证

- `go test ./internal/service/system -run AIMemory`
  - 覆盖 policy 权限、覆盖优先级、来源过滤、TTL、原因码。
- `go test ./internal/repository/system -run AIMemory`
  - 覆盖 document 字段迁移、summary 读取契约收紧、现有 upsert 行为不回归。
- 如有必要，补充 `flag.SQL()` / AutoMigrate 相关索引与字段存在性验证。

# 风险

- `ConversationSummary` 读取接口收紧后，所有调用点都必须同步改签名，否则会出现编译或行为回归。
- document 去重字段落库后，若后续写入链路未统一生成规则，容易出现“字段存在但未稳定使用”的半成品状态。
- policy 规则如果把“权限失败”和“内容读取失败”混在一起，会破坏 fail-closed 语义，需要测试明确区分。

# 执行顺序

1. 收口 domain/service 层治理类型与原因码。
2. 实现 `aiMemoryPolicy` 规则函数与单测。
3. 扩展 `AIMemoryDocument` 字段与 repository upsert/query。
4. 收紧 `ConversationSummary` repository 接口与测试。
5. 视需要补 service 侧最小接线与编译修正。
6. 运行定向测试并回报结果。

# 待确认

- 这次实现默认只完成“治理规则 + repository 契约 + 测试”，不把 memory 正式接入 `aiSvc` 主链路 writeback/recall。
- document 去重字段本次先保证“字段、生成规则、测试”到位；若现有写入入口尚未落地，不额外扩项到 Qdrant 或 embedding 流程。
