可以。我按“为什么这么分层”来讲，不只说文件名，也说每层承担什么职责、重要字段为什么这样设计、函数为什么这样写。

**整体设计**
这次我做的不是“完整记忆系统”，而是 **Phase 1 冻结骨架**。目标只有两个：

- 先把记忆系统的 **稳定语义** 定下来：`facts / documents / conversation_summary`、`scope / visibility`、配置口径、Repository/Service 契约。
- 先把 **正式落库能力** 建好，但 **不接入当前 AI 主链路**，避免现在就改变对话行为。

所以你会看到这版代码的特点是：

- `domain` 先定义语义和 helper。
- `entity` 先定义三张表。
- `repository` 先把读写规则落稳。
- `service` 只放骨架，不真正 recall/writeback。
- `config` 和 `migration` 先接好，保证后面阶段能直接接力。

---

**1. 目录结构为什么这么拆**
这次新增主要落在这几个目录：

- [internal/domain/ai/memory.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:9)
作用：定义“记忆是什么”的语义层，不依赖 GORM、Gin、Qdrant。
- [internal/model/entity/ai_memory_fact.go](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:6)
作用：定义 `fact` 表结构。
- [internal/model/entity/ai_memory_document.go](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:10)
作用：定义 `document` 表结构。
- [internal/model/entity/ai_conversation_summary.go](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:6)
作用：定义 `conversation summary` 表结构。
- [internal/repository/interfaces/aiMemoryRepository.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/interfaces/aiMemoryRepository.go:11)
作用：冻结 memory 仓储接口。
- [internal/repository/system/aiMemoryRepo.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:17)
作用：GORM 版 memory repository 实现。
- [internal/service/system/aiMemorySvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:28)
作用：记忆服务骨架，后续 recall/writeback 从这里长出来。
- [internal/service/system/aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:4)
作用：给下一步“记忆治理”预留固定位置。
- [internal/model/config/ai.go](/D:/workspace_go/test/go/personal_assistant/internal/model/config/ai.go:4)
作用：冻结 `AI.Memory` 配置结构。
- [internal/model/config/qdrant.go](/D:/workspace_go/test/go/personal_assistant/internal/model/config/qdrant.go:9)
作用：冻结 Qdrant 的 knowledge/memory collection 配置。
- [internal/model/config/config.go](/D:/workspace_go/test/go/personal_assistant/internal/model/config/config.go:330)
作用：把 Viper 配置真正读到结构体。
- [internal/core/config.go](/D:/workspace_go/test/go/personal_assistant/internal/core/config.go:65)
作用：默认值和 env binding。
- [internal/repository/system/supplier.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/supplier.go:13)
作用：把 memory repo 接进系统 supplier。
- [internal/repository/system/supplierImpl.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/supplierImpl.go:63)
作用：提供 `GetAIMemoryRepository()`。
- [flag/flagSql.go](/D:/workspace_go/test/go/personal_assistant/flag/flagSql.go:30)
作用：把三张新表纳入 AutoMigrate。
- [internal/core/config_memory_test.go](/D:/workspace_go/test/go/personal_assistant/internal/core/config_memory_test.go:12)
作用：验证配置和兼容口径。
- [internal/repository/system/aiMemoryRepo_test.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo_test.go:17)
作用：验证 repository 语义。

这套拆法的核心原因是：

- `domain` 负责稳定语义。
- `entity` 负责落库形状。
- `repository` 负责持久化规则。
- `service` 负责未来业务编排。
- `config` 负责运行时开关。
- `flag` 负责建表。

这样后面你做 `治理 / writeback / 压缩恢复 / RAG`，不会再反复改层次。

---

**2. domain 层是怎么设计的**
[internal/domain/ai/memory.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:9) 这一个文件，承担了 4 件事。

第一，冻结枚举语义：

- `MemoryScopeType` 在 [memory.go:9](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:9)
- `MemoryType` 在 [memory.go:18](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:18)
- `MemoryVisibility` 在 [memory.go:31](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:31)

我这里把 `scope` 和 `visibility` 明确拆开，是因为它们不是一回事：

- `scope` 解决“这条记忆属于谁”
- `visibility` 解决“谁可以读它”

这是你后面做权限过滤时最重要的基础。

第二，冻结业务 namespace 常量。
同文件里我把 `user_preference / oj_profile / oj_goal / org_profile / org_learning_pattern / ops_incident / ops_runbook` 固定下来。这样后面写入和召回不会到处散落魔法字符串。

第三，统一生成 `scope_key`。
核心函数是：

- [BuildMemoryScopeKey](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:64)
- [BuildConversationMemoryScopeKey](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:84)

这里我刻意不让业务代码自己拼 `"self:user:123"`，而是统一走 helper。原因很直接：

- 防止格式漂移
- 防止后面清理、查询、权限判断时出现多种 key 口径
- 让 summary/fact/document 共用同一套 scope 规则

第四，定义 query struct：

- [MemoryFactQuery](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:93)
- [MemoryDocumentQuery](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:102)

我不用一长串参数，而用 query struct，是为了给后面“治理、混合召回、排序、分页、过滤扩展”留空间，不然 Repository 很快就会签名爆炸。

---

**3. 三个实体为什么这样分**
**AIMemoryFact**
定义在 [ai_memory_fact.go](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:6)。

它表示“结构化、稳定、可覆盖”的事实。最重要的字段是：

- `ScopeKey` [line 8](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:8)
表示归属范围。
- `ScopeType` [line 9](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:9)
保存枚举值，避免每次靠解析 `scope_key` 字符串判断类型。
- `Visibility` [line 10](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:10)
表示访问等级。
- `UserID` / `OrgID`
虽然 `scope_key` 已经能表达归属，但我还是保留这两个字段，目的是方便权限校验、清理、调试和后台查询。
- `Namespace` [line 13](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:13)
表示它属于哪个业务域。
- `FactKey` [line 14](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:14)
表示这个 namespace 下的具体键。
- `FactValueJSON` [line 15](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:15)
事实值本体。
- `Summary` [line 16](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:16)
给模型和调试看的可读摘要。
- `Confidence` [line 17](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:17)
后面治理会用它判断是否允许写入。
- `SourceKind` / `SourceID` [line 18-19](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:18)
保存来源证据链。
- `EffectiveAt` / `ExpiresAt` [line 20-21](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_fact.go:20)
后面做新鲜度和过期策略靠它。

为什么 `Fact` 不做软删除？
因为你这一步已经定了：它是“唯一键覆盖更新”的模型。`fact` 更像当前有效状态，不像历史文档，所以我保留唯一键覆盖，不引入软删除复杂度。

**AIMemoryDocument**
定义在 [ai_memory_document.go](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:10)。

它表示“文本型、可召回”的长期记忆。最重要字段：

- `MemoryType` [line 17](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:17)
区分 `semantic / episodic / procedural / incident / faq`
- `Topic` [line 18](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:18)
给轻量主题过滤和排序用
- `Title` / `Summary` / `ContentText` [line 19-21](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:19)
这是召回文本的三层表达
- `Importance` / `QualityScore` [line 22-23](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:22)
后面召回排序和治理会用
- `EmbeddingModel` [line 24](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:24)
记录向量模型
- `QdrantPointID` [line 25](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:25)
你要求它只是单 chunk 兼容字段，所以我只保留，不扩 chunk 表
- `DeletedAt` [line 32](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_memory_document.go:32)
这里保留软删除，因为 document 天然更像“可归档文本”，后面人工失效、治理清理都会更自然。

**AIConversationSummary**
定义在 [ai_conversation_summary.go](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:6)。

它表示“当前会话的压缩结果”，不是长期知识。关键字段：

- `ConversationID` [line 7](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:7)
直接做主键，因为一个会话只保留一份当前摘要。
- `UserID` / `OrgID` / `ScopeKey` [line 8-10](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:8)
这是你特别要求增加的，我也认为很对。后面权限校验、后台清理、调试都需要。
- `CompressedUntilMessageID` [line 11](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:11)
这个字段非常关键，它定义了“摘要已经覆盖到哪里”。
- `SummaryText` [line 12](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:12)
给模型读的主体摘要。
- `KeyPointsJSON` / `OpenLoopsJSON` [line 13-14](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:13)
后面恢复上下文时会非常有用，一个表示关键结论，一个表示未完成事项。
- `TokenEstimate` [line 15](/D:/workspace_go/test/go/personal_assistant/internal/model/entity/ai_conversation_summary.go:15)
后面调优压缩效果要靠它。

---

**4. Repository 为什么这样写**
接口先定义在 [aiMemoryRepository.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/interfaces/aiMemoryRepository.go:11)。

这一层我只冻结最小能力：

- `WithTx`
- `UpsertFact`
- `ListFacts`
- `BatchUpsertDocuments`
- `ListDocuments`
- `GetConversationSummary`
- `UpsertConversationSummary`

这套接口的思路很明确：

- `Fact` 是单条覆盖更新
- `Document` 未来一般是批量写入
- `Summary` 是按会话读写

具体实现都在 [aiMemoryRepo.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:17)。

几个核心函数：

- [NewAIMemoryRepository](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:22)
标准构造。
- [WithTx](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:27)
为什么参数是 `any`？
因为你现有仓库事务接口就是 `Group.InTx(ctx, func(tx any) error)` 这一套，我这里跟现有风格保持一致，避免 memory 成为特例。
- [UpsertFact](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:35)
这里我用了 `OnConflict`，冲突列就是 `scope_key + namespace + fact_key`。这和你冻结的“唯一键覆盖更新”完全一致。
- [ListFacts](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:66)
默认过滤 `expires_at`，并要求必须传 `ScopeKeys + AllowedVisibilities`。这是我在代码层落实“查询必须同时校验 scope 和 visibility”。
- [BatchUpsertDocuments](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:96)
这里按 `id` 做 upsert，并且把 `deleted_at = nil`，意思是文档如果同 ID 被重新写入，会自动“恢复生效”。
- [ListDocuments](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:135)
默认依赖 GORM 软删除过滤，同时再过滤过期数据。
- [GetConversationSummary](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:165)
按 `conversation_id` 读。
- [UpsertConversationSummary](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:180)
按 `conversation_id` 覆盖更新，符合“每个会话只有一份当前摘要”的设计。

---

**5. Service 为什么只做骨架**
[aiMemorySvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:28) 现在是“冻结服务边界”，不是完整业务逻辑。

关键点：

- [AIMemoryService](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:28)
只持有 `repo + outboxRepo + policy`
- [NewAIMemoryService](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:35)
不是自己 `new repo`，而是从正式的 `repository.Group.SystemRepositorySupplier` 取，这符合你项目的依赖组织方式。
- [Recall](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:47)
- [OnTurnCompleted](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:54)
- [RefreshConversationSummary](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:61)

这 3 个现在都返回 `errAIMemoryPhase1NotImplemented`，是故意的。因为这版不能改当前 AI 行为，只能冻结接口。

为什么方法签名这样定？
因为它们要对齐你现有 AI 接入点：

- recall 输入已经在 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:11) 里有 `aiMemoryRecallInput`
- 未来读侧接入点在 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:78)
- future writeback 收尾点在 [aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:436)

也就是说，这个 skeleton 不是空想，它是按你现有 AI 主链路预留出来的。

目前真正可用的是两个 pass-through：

- [UpsertFact](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:70)
- [ScheduleDocumentUpsert](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:78)

它们先让后续阶段有地方接入，但现在不改变主逻辑。

[aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:4) 现在只放了空壳，也是故意的：下一步“记忆治理”就从这里长，不会再另起炉灶。

---

**6. 配置层为什么这么写**
配置分 3 层：

- 结构体定义
- Viper 读取
- 默认值和 env 绑定

`AI.Memory` 结构体在 [ai.go](/D:/workspace_go/test/go/personal_assistant/internal/model/config/ai.go:18)。

这些字段本质上是在冻结后续能力边界：

- `RecallTopK`
- `RecallMaxChars`
- `RecentRawTurns`
- `CompressThresholdTokens`
- `SummaryRefreshEveryTurns`
- `WritebackAsync`
- `EnableEntityMemory`
- `EnableLongTermMemory`
- `EnableOrgMemory`
- `EnableOpsMemory`
- `MinImportance`
- `EmbedModel`

意思是：后面你做 recall、压缩、writeback、RAG 时，不要再临时硬编码行为。

Qdrant 结构体在 [qdrant.go](/D:/workspace_go/test/go/personal_assistant/internal/model/config/qdrant.go:9)。

这里最关键的是：

- `CollectionName` [line 17](/D:/workspace_go/test/go/personal_assistant/internal/model/config/qdrant.go:17)
保留旧口径兼容
- `KnowledgeCollectionName` [line 20](/D:/workspace_go/test/go/personal_assistant/internal/model/config/qdrant.go:20)
新知识库 collection
- `MemoryCollectionName` [line 23](/D:/workspace_go/test/go/personal_assistant/internal/model/config/qdrant.go:23)
新记忆 collection，但这一阶段只保留配置位，不初始化

真正的兼容逻辑在 [config.go](/D:/workspace_go/test/go/personal_assistant/internal/model/config/config.go:346)。

这里我专门做了这件事：

- 如果显式设置了 `qdrant.knowledge_collection_name`，优先用新键
- 如果没有显式设置，就回退到旧 `qdrant.collection_name`

这个细节是必须的，不然你一旦给新键设默认值，旧键兼容就失效了。这个兼容判断在 [hasExplicitConfigValue](/D:/workspace_go/test/go/personal_assistant/internal/model/config/config.go:476)。

默认值和 env 绑定都在 [internal/core/config.go](/D:/workspace_go/test/go/personal_assistant/internal/core/config.go:65)。

你能看到：

- 默认值从 [line 65](/D:/workspace_go/test/go/personal_assistant/internal/core/config.go:65) 开始
- env binding 从 [line 280](/D:/workspace_go/test/go/personal_assistant/internal/core/config.go:280) 开始

这样后面切环境时不需要改代码。

---

**7. supplier 和 migration 为什么要改**
supplier 的修改很简单，但很重要。

- 接口扩展在 [supplier.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/supplier.go:13)
- 创建 repo 在 [supplier.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/supplier.go:79)
- 存进 supplier struct 在 [supplier.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/supplier.go:150)
- getter 在 [supplierImpl.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/supplierImpl.go:64)

这么做的原因只有一个：

**memory repo 必须成为正式系统依赖，而不是临时自己 new 的旁路对象。**

否则后面事务、测试、依赖注入都会不统一。

迁移入口在 [flagSql.go](/D:/workspace_go/test/go/personal_assistant/flag/flagSql.go:30)。

这里我只做了一件事：把 3 个模型放进 AutoMigrate。因为这一阶段目标是“正式落库”，不是单独写一套 migration runner。

---

**8. 测试怎么覆盖的**
配置测试在 [config_memory_test.go](/D:/workspace_go/test/go/personal_assistant/internal/core/config_memory_test.go:12)。

它主要验证两件事：

- `AI.Memory` 的 env binding 生效
- `qdrant.collection_name` 到 `knowledge_collection_name` 的兼容回退生效

repository 测试在 [aiMemoryRepo_test.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo_test.go:17)。

它覆盖了 5 件事：

- 三张表和关键索引能迁出来
- `Fact` 能按唯一键覆盖更新
- `ListFacts` 默认过滤过期数据
- `Document` 默认过滤过期和软删除
- `ConversationSummary` 能按 `conversation_id` 覆盖更新，并保留 `user_id/org_id/scope_key`

这组测试其实就是在保护你这次冻结下来的规则。

---

**9. 这版刻意没做什么**
这部分也很重要，不然容易误解为“记忆系统已经完成”。

这版没做：

- 没把 memory 接到 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:78) 的正式 recall
- 没把 memory 接到 [aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:436) 的正式 writeback
- 没做记忆治理规则
- 没做 document chunk 表
- 没做 embedding / Qdrant upsert
- 没做混合召回
- 没做 admin API / delete API

所以这版的准确定位是：

**把“记忆系统的地基”浇筑好，而不是把整栋楼盖完。**

---

**10. 你接下来怎么看这套代码**
如果你要理解后续开发顺序，可以直接按这条链看：

1. 先看 [memory.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory.go:9)，理解语义和 scope/visibility 口径  
2. 再看三个 entity，理解三张表各自装什么  
3. 再看 [aiMemoryRepository.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/interfaces/aiMemoryRepository.go:11) 和 [aiMemoryRepo.go](/D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:22)，理解落库规则  
4. 再看 [aiMemorySvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:28)，理解后续 recall/writeback 会从哪里接  
5. 最后看测试，理解我真正冻结了哪些行为

如果你要，我下一条可以继续按同样方式，专门给你讲：

- `AIMemoryFact / AIMemoryDocument / AIConversationSummary` 这 3 张表该怎么在你业务里使用
- 或者直接讲下一步 `记忆治理` 应该怎么接到这套骨架上