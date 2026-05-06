从当前代码看，你这个“RAG 召回”模块已经不是概念层了，而是一条比较完整的链路：**把用户当前 query 转成向量，从 Qdrant 里按权限边界召回 chunk，再回表拿正文，最后把结果作为一段“记忆上下文”注入到 AI runtime。**  
主入口在 [aiMemoryRecall.go](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:28)，接入上下文组装在 [aiContext.go](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:64)。

**先用一句人话讲清它在做什么**
你不是让 AI 每次都去翻完整历史，也不是让它直接扫数据库全文。  
你做的是：

1. 先把“这次用户想问什么”编码成向量。
2. 用这个向量去长期知识库里找最相近的知识片段。
3. 把这些片段按权限、分数、模型版本再过滤一遍。
4. 最后把它们整理成一段简洁的背景材料，塞回本轮上下文。

所以它的核心价值不是“查得快”，而是**在不把历史全塞进 prompt 的前提下，把真正相关的长期知识捞回来。**

**完整链路是怎么走的**

1. **上下文组装阶段触发召回**
在 [aiContext.go](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:64) 的 `Build` 里，系统先拿到当前会话的历史消息，然后如果挂了 `memory` provider，就会调用 `RecallMessages(...)`。  
也就是说，RAG 召回不是一个独立接口，而是 **runtime 上下文构建的一部分**。

2. **召回不是只查向量，还会同时恢复 summary 和 facts**
在 [aiMemoryRecall.go](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:28) 的 `Recall` 里，实际做了三类恢复：

- 会话摘要 `summary`
- 稳定事实 `facts`
- 长期文档 `ragItems`

所以你这个模块的设计不是“只有向量召回”，而是 **短期摘要记忆 + 结构化事实 + 长期文档 RAG** 三路合流。  
RAG 只是其中“长期知识”这一层。

3. **先把当前 query 做 embedding**
真正的长期文档召回在 [recallLongTermDocuments](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:126)。  
这里先做几层前置判断：

- 长期记忆功能是否开启
- query 是否为空
- `embedder` 和 `vectorSearcher` 是否可用
- `userID` 是否有效

通过后，调用 [embedder.go](D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/memory/embedder.go:59) 里的 `Embed`，把当前 query 转成一个向量。  
而且这里用的是**和建索引时同一套 embedding 模型和维度**，这一点很重要，否则 query 向量和 chunk 向量根本不在一个空间里。

4. **用 query vector 去 Qdrant 检索**
向量检索接口定义在 [memory_rag.go](D:/workspace_go/test/go/personal_assistant/internal/domain/ai/memory_rag.go:60)，真正实现是在 [qdrant_store.go](D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/memory/qdrant_store.go:83) 的 `SearchChunks`。

你这里传给 Qdrant 的不是“裸向量 + topK”，而是带过滤条件的检索请求：

- `scope_key`
- `visibility`
- `user_id`
- `limit`
- `min_score`

过滤器构造在 [qdrant_store.go](D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/memory/qdrant_store.go:152)。

这一步很关键，因为它说明你不是“先查出来再看能不能读”，而是**在检索阶段就把权限边界带进去**。  
当前实现先收敛在 `self` 作用域，也就是个人长期知识：

- `scope_key = self:user:{userID}`
- `visibility = self`
- `user_id = 当前用户`

这其实是一个很稳的第一版，先把个人知识召回链路打通，再考虑组织级知识。

5. **Qdrant 返回的只是候选点，不直接信任**
Qdrant 搜出来后，返回的是一组 `pointID + score`，不是最终要注入 prompt 的正文。  
你后面又做了一层**数据库回表校验**，在 [aiMemoryRecall.go](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:174) 调 `ListDocumentChunksByPointIDs(...)`，仓储实现见 [aiMemoryRepo.go](D:/workspace_go/test/go/personal_assistant/internal/repository/system/aiMemoryRepo.go:304)。

这里回表的意义非常大：

- 过滤掉已过期 document
- 过滤掉已软删除 document
- 校验 chunk 的 `embedding_model`
- 校验 `embedding_dimension`
- 再次校验 `scope_key / visibility / user_id`

也就是说，你不是“Qdrant 查到了就直接信”，而是把 Qdrant 当成**召回加速层**，真正要进上下文的内容，仍然回到业务库做二次确认。  
这保证了数据一致性，也降低了向量库和业务数据漂移带来的风险。

6. **按分数顺序组装长期知识片段**
回表成功后，会按 Qdrant 返回顺序保留候选 chunk，并带上分数，变成 `aiMemoryRAGRecallItem`。  
最终渲染逻辑在 [renderAIMemoryRAGSection](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:258)。

你没有把它做成一坨原始文本，而是渲染成这样的结构：

- `[memoryType/topic score=0.xxx] chunk内容`

这个设计很好，因为它保留了最重要的辅助信息：

- 这段知识属于什么类型
- 主题是什么
- 相似度大概有多高

这样既方便调试，也方便后续做更细的 prompt 控制。

7. **最后不是直接返回给前端，而是注入成一条 memory message**
所有召回结果最终会被拼成一条特殊的 assistant message，见 [buildAIMemoryContextContent](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:207)。  
内容结构大概是：

- `Conversation Summary`
- `Stable Facts`
- `Long-term Documents`
- `Current Query`

然后以一条 `memory_context_xxx` 消息的形式插入 runtime history。  
也就是说，RAG 召回结果在你的系统里扮演的是**系统恢复的背景上下文**，不是直接展示给用户的最终答案。

**这个模块里几个很重要的设计点**

- **RAG 是 fail-open 的**  
  在 [recallLongTermDocumentsFailOpen](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:108)，如果 embedding 或向量检索失败，只记日志，不阻断主链路。  
  这非常像生产系统思路：RAG 是增强能力，不应该因为它挂了就让整个 AI 对话不可用。

- **召回结果有最小分数门槛**  
  `minScore` 是配置项，默认值在 [aiMemoryRecall.go](D:/workspace_go/test/go/personal_assistant/internal/service/system/aiMemoryRecall.go:16)。  
  低于阈值的 chunk 不会注入，避免把“沾点边但其实没用”的内容喂给模型。

- **召回内容还有长度预算**
  长期文档 section 会被单独裁剪，避免 RAG 片段把 prompt 撑爆。  
  这说明你不是“召回越多越好”，而是在控制 token 成本和噪声。

- **召回和压缩是连着的**
  回忆消息注入后，还会走 `CompressMessages(...)`，只保留 memory + recent turns。  
  所以整个设计目标不是“把更多东西放进上下文”，而是**用更少但更值钱的信息替代冗长历史**。

**如果把它讲成面试里的“我是怎么做的”**
你可以这样说：

> 我把 RAG 召回做成了 AI 上下文组装阶段的一部分，而不是一个独立查询接口。系统在进入 runtime 前，会先恢复会话摘要和稳定事实，再把当前 query 用和建索引阶段一致的 embedding 模型转成向量，去 Qdrant 按 `scope_key`、`visibility`、`user_id` 做带权限边界的向量检索。拿到候选 point 之后，我没有直接把 Qdrant 返回结果喂给模型，而是回表查询 chunk 元数据，再校验模型版本、维度、权限和文档有效期，最后按分数顺序把长期知识片段整理成一段 memory context 注入到历史消息里。整个链路是 fail-open 的，RAG 失败不会影响主对话，只是退化成 summary/fact 记忆。  

**一句最精炼的总结**
你这个模块本质上是在做：  
**“带权限过滤和二次校验的 query-driven 长期知识召回，并把召回结果转成可控的上下文记忆，而不是把向量检索结果直接裸塞给模型。”**

如果你要，我下一条可以继续帮你把这个点压成一版更适合简历/面试的 `30 秒版` 和 `1 分钟版`。