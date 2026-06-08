# 目标

实现记忆模块 Phase 1 冻结版基础骨架，正式落库 `facts / documents / conversation_summary` 三类模型，冻结 scope/visibility、配置、repository/service 契约，并把 memory repository 接入现有 supplier 体系。

# 范围

- 新增 memory domain 类型与 scope helper。
- 新增 3 个 memory entity、repository interface/impl、service 骨架。
- 扩展 AI/Qdrant 配置和加载逻辑。
- 扩展 `flag.SQL()` 自动迁移与索引创建。
- 扩展 repository supplier 接口与实现。
- 增加最小配置、repository、迁移回归测试。

# 改动

- `internal/domain/ai`
  - 增加 memory 类型、visibility、scope helper、query struct。
- `internal/model/entity`
  - 新增 `AIMemoryFact`、`AIMemoryDocument`、`AIConversationSummary`。
- `internal/model/config`
  - `AI` 增加 `Memory` 子配置。
  - `Qdrant` 增加 `KnowledgeCollectionName`、`MemoryCollectionName`，保留兼容字段。
- `internal/repository/interfaces`
  - 新增 `AIMemoryRepository`。
- `internal/repository/system`
  - 新增 memory repository。
  - supplier / setup / impl 接入 `GetAIMemoryRepository()`。
- `internal/service/system`
  - 新增 `AIMemoryService`、`aiMemoryPolicy` 骨架。
- `flag/flagSql.go`
  - 纳入新表迁移与 memory 索引创建。
- `tests`
  - 补配置、repository、迁移回归测试。

# 验证

- `go test` 覆盖新增 repository/config/service 基础测试。
- 验证 `flag.SQL()` 后 3 张表和关键索引存在。
- 验证现有 AI 会话相关构造和 repository group 初始化不回归。

# 风险

- Qdrant 配置兼容处理不当会影响现有 collection 初始化。
- supplier 接口变更会影响现有依赖构造，需保证全链路编译通过。
- `AIMemoryFact` 不使用软删除，覆盖更新逻辑必须稳定。

# 执行顺序

1. 补 plan 文件并转已审状态。
2. 实现 memory domain/entity/config。
3. 实现 repository interface/impl 与 supplier 接入。
4. 实现 service 骨架。
5. 扩展 `flag.SQL()` 迁移与索引。
6. 增加测试并执行验证。

# 待确认

- 无；按已确认方案直接实施。
