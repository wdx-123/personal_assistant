记忆治理现在主要做在 **policy 层**，不是写在 Repository 里。核心文件是 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:11)，它负责在 writeback 真正入库前判断：能不能存、归到哪个 scope、谁能读写、多久过期、能不能覆盖旧值。

整体分成 4 块：

1. **候选内容建模**

   在 [memory_policy.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory_policy.go:51) 定义了治理需要的输入结构：

   - `MemoryFactCandidate`：待写入的结构化事实，比如用户偏好、OJ 画像、目标。
   - `MemoryDocumentCandidate`：待写入的长期文本，比如 FAQ、runbook、incident 摘要。
   - `MemoryAccessContext`：当前主体和授权上下文，包括 `Principal`、已批准的 org scope、是否允许 platform ops。
   - `MemoryDecision`：统一返回 `Allowed / ReasonCode / Reason`，方便后面日志和调试。

2. **写入准入**

   `ShouldStoreFact` 在 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:14) 做 fact 准入：

   - 禁止原始 trace、完整工具输出这类来源。
   - 如果和实时业务真相冲突，拒绝。
   - 低价值、空 namespace、空 fact key、空 value，拒绝。
   - 然后依次过 `ResolveScope -> ResolveVisibility -> CanWriteMemory`。

   `ShouldStoreDocument` 在 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:63) 做 document 准入：

   - `session_summary` 不能作为 document 存，要走 conversation summary。
   - 禁止 raw trace / full tool output。
   - 低价值、空摘要且空正文，拒绝。
   - 通过权限后，会生成 `ContentHash / SummaryHash / DedupKey` 做去重。

3. **权限和 scope**

   `ResolveScope` 在 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:137)：

   - `self`：必须有当前用户，且候选 userID 不能和 principal 不一致。
   - `org`：必须传明确 orgID，并且这个 org 必须在 `ApprovedOrgScopeKeys` 或 `ApprovedOrgIDs` 里。
   - `platform_ops`：必须显式允许 `AllowPlatformOps`，且主体必须是超管。

   `ResolveVisibility` 在 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:232) 固定映射：

   - `self -> self`
   - `org -> org`
   - `platform_ops -> super_admin`

   读写权限最后都走 `evaluateMemoryAccess`，也就是 fail closed：缺授权信息就拒绝，不会默认放行。

4. **过期、覆盖、去重**

   `ResolveTTL` 在 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:273)：

   - `user_preference / org_profile / ops_incident / ops_runbook` 默认长期有效。
   - `oj_goal` 30 天。
   - `oj_profile` 60 天。
   - `org_learning_pattern` 14 天。
   - summary 不靠 TTL。

   `ShouldOverrideFact` 在 [aiMemoryPolicy.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryPolicy.go:330)：

   - 候选值为空拒绝。
   - 和当前值一样则跳过。
   - 按来源优先级覆盖：
     - 个人记忆：用户显式声明 > 管理员设置 > 工具验证摘要 > 模型推断。
     - 组织/平台记忆：管理员设置 > 用户显式声明 > 工具验证摘要 > 模型推断。

   Document 去重键在 [memory_policy.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory_policy.go:173)：优先用 `source_kind + source_id + topic`，否则退到 summary hash，再退到 content hash。

当前状态有一个关键点：**治理规则已经实现并有测试，但还没有真正接入主 writeback 链路。**  
[AIMemoryService](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemorySvc.go:55) 里的 `Recall / OnTurnCompleted / RefreshConversationSummary` 现在仍是 phase 1 skeleton，返回 `errAIMemoryPhase1NotImplemented`。也就是说，治理能力已经准备好了，下一步要在 `memory writeback hook` 里把这些 policy 调起来，再允许入 Repository。