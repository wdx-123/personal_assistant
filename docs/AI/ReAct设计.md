这次实现的核心思路其实很简单：

**我没有去“发明一个新框架”，而是把你现有的三层边界补完整，让它稳定地跑单 Agent ReAct。**

也就是：

- `domain/ai` 继续只定义协议
- `service/system` 继续负责业务编排、工具过滤、上下文装配
- `infrastructure/ai/eino` 继续负责模型执行和 tool loop
- `aiSink + aiProjector` 继续负责 SSE 和消息落库

所以你现在得到的不是“推翻重写”，而是“把原来已有骨架补成可交付版本”。

---

**一、我到底实现了什么**

这次主要做了 4 件事。

1. 把“上下文装配”从 `AIService` 里抽出来，变成单独协作者。  
入口还是 [aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:223)，但它现在不再自己直接拼 `History + DynamicSystemPrompt`，而是交给 `contextAssembler`，[aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:38)、[aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:319)。

2. 预埋了后续扩展点，但默认不启用。  
也就是：
- `aiMemoryProvider`
- `aiContextCompressor`
- `aiPromptBuilder`

这些都在 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:55) 和 [aiPrompt.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiPrompt.go:7)。  
默认没注入时，行为和你之前基本一致。

3. 把 ReAct runtime 固化成两条路径。  
在 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:79)：
- 没工具：走 `streamTextOnly`
- 有工具：走 `streamWithTools`

这样逻辑边界就很清楚，不会把普通对话和 ReAct 混成一锅。

4. 给未来的 LLM 网关留了注入口。  
新增了 `ChatModelFactory`，定义在 [options.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/options.go:12)，实际使用在 [chat_model.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/chat_model.go:30)。  
现在不用它也没关系，但以后你做网关、模型路由、熔断，就不需要改 service 层。

---

**二、为什么要这样做**

因为你现在真正需要的不是“更多功能”，而是**把控制面和执行面分开**。

之前 `AIService` 已经做很多事情了：
- 查会话
- 查历史
- 创建消息
- 过滤工具
- 拼 prompt
- 调 runtime
- 收尾

这没错，但“上下文构造”已经开始变成一个独立职责了。你后面要加：
- memory recall
- 上下文压缩
- prompt builder
- 甚至未来 query rewrite

如果这些还堆在 `AIService` 里，`StreamConversation` 会越来越难看。

所以我把它抽成了：

- `AIService` 负责调度
- `aiContextAssembler` 负责构造 runtime 输入

这一步是这次实现里最重要的结构变化。

---

**三、一次请求现在是怎么跑的**

你可以把当前完整链路理解成这 8 步。

### 1. Controller 调到 `AIService.StreamConversation`

主入口还是 [aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:223)。

这里做的是业务入口校验：
- `req` 不能为空
- 路径上的 `conversationID` 和 body 里的要一致
- `writer` 不能是空
- `runtime` 不能是空

然后读取：
- 会话
- 用户
- 历史消息

---

### 2. 创建本轮 user/assistant 消息骨架并落库

这一步还在 `AIService` 里。

会先创建两条消息：
- 用户消息：直接 `success`
- assistant 消息：先是 `loading`

然后调用 `persistStreamStart(...)` 做事务化写入。  
这样做的目的，是在真正调用模型前，数据库里就已经有这轮对话的“骨架”。

好处是：
- 前端列表页能立即看到“生成中”
- 后续就算流中断，库里也有 assistant 占位消息可追踪

---

### 3. 创建 sink

这里创建的是 [aiSink.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSink.go:16) 里的 `aiStreamSink`。

它的职责非常关键：

- 往 SSE 发事件
- 把这些事件折叠成 assistant 消息状态
- 定时落库

也就是说 runtime 根本不需要知道数据库和 SSE，它只管发 `aidomain.Event`。  
这就是你现在这套设计最好的地方之一。

---

### 4. 构造工具调用上下文和 principal

在 `AIService` 里会构造：

- `AIToolPrincipal`
- `ToolCallContext`

`principal` 只保存最小授权事实：
- `UserID`
- `CurrentOrgID`
- `IsSuperAdmin`

而不是塞一大堆业务对象。  
这个定义在 [tool.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/tool.go:61)。

这一步的意义是：  
**service 把“谁在调用”准备好，tool runtime 只消费这个最小事实。**

---

### 5. 过滤本轮可见工具

调用的是 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:190) 的 `FilterVisibleTools(...)`。

这里做的是“本轮可见性过滤”，不是最终执行授权。  
比如：
- self only
- org capability
- super admin only

只把当前用户**看得见**的工具暴露给模型。

为什么要这样做？
因为如果你把所有工具都给模型，它会尝试调用本来不该看到的工具，失败率会很高。

所以现在是两层保护：
1. 可见性过滤：决定模型能不能看见
2. 执行前二次鉴权：决定工具能不能真正执行

这点你原来的设计就很好，我保留了。

---

### 6. 由 `contextAssembler` 统一构造 runtime 输入

这是这次新加的核心点。

位置在 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:79)。

它现在做三件事：

1. 把 DB 历史消息转成 `aidomain.Message`
2. 可选追加 memory recall
3. 可选做上下文压缩
4. 生成动态 prompt

它的返回值是：

- `History`
- `DynamicSystemPrompt`

也就是最后真正喂给 runtime 的上下文快照。

默认行为非常保守：
- `Memory == nil`：不召回
- `Compressor == nil`：不压缩
- `PromptBuilder == nil`：走默认 prompt builder

所以这次改动不会改变你现在线上行为。

---

### 7. runtime 执行 ReAct

入口在 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:79)。

它先发一个：
- `conversation_started`

然后分成两条路：

#### 路径 A：无工具
走 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:110) 的 `streamTextOnly(...)`

流程是：
- 构造 messages
- 调模型流式输出
- 每个 chunk 发 `assistant_token`
- 结束时发 `message_completed`
- 最后发 `done`

#### 路径 B：有工具
走 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:164) 的 `streamWithTools(...)`

流程是：
1. 绑定本轮可见工具到模型
2. 构造完整 messages
3. 模型先回答一轮 assistant
4. 如果 assistant 没有 tool call：
   - 直接进入最终回答
5. 如果 assistant 有 tool call：
   - 顺序执行工具
   - 把 tool result 转成 `ToolMessage`
   - 再塞回 messages
   - 模型继续下一轮

这就是标准单 Agent ReAct loop。

---

### 8. sink/projector 折叠事件并落库

runtime 发出来的不是 HTTP 响应，也不是 DB 更新，而是 `aidomain.Event`。

这些事件进入 [aiSink.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSink.go:45) 的 `Emit(...)` 后，会发生两件事：

1. 写到 SSE
2. 调 `projector.applyEvent(...)`

投影逻辑在 [aiProjector.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiProjector.go:104)。

它会根据事件更新 assistant 消息快照：

- `assistant_token`
  追加正文
- `tool_call_started`
  创建 running trace item
- `tool_call_finished`
  更新 trace item 为 success/failed
- `message_completed`
  覆盖最终正文并标记 success
- `error`
  写错误文案并标记 error

最终 `trace_items_json` 和 `content/status/error_text` 都会落到消息表里。

---

**四、动态 prompt 是怎么工作的**

默认 prompt 逻辑还在 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:275)。

它现在固定会告诉模型：

- 本轮只能用列出的工具
- 缺少 `org_id/task_id/execution_id/request_id` 时不要猜
- 工具可见不等于一定能执行成功
- 当前组织上下文是什么
- 每个工具参数是什么

这个 prompt 的目标不是“教模型变聪明”，而是**降低错误调用率**。

举个例子：

如果用户说“帮我查排名”，而工具要求 `platform` 必填，模型此时应该：
- 不去乱猜 `leetcode`
- 不去乱调工具
- 而是直接自然追问用户

这个行为在测试里已经覆盖了，[runtime_tools_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime_tools_test.go:231)。

---

**五、工具执行阶段我做了哪些细节处理**

工具执行在 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:340)。

这里有几个关键点：

1. `tool name` 会先 `TrimSpace`
避免模型输出名字前后有空格导致匹配失败。

2. `ArgumentsJSON` 原文完整透传
不会在 runtime 层先偷偷改参数结构，工具自己解析。

3. 每次工具调用都会发两种事件
- `tool_call_started`
- `tool_call_finished`

这样前端 trace 能稳定显示运行态和结束态。

4. 工具失败时，先发失败 trace，再返回 error
这点很重要。  
如果工具失败但没有 `finished(failed)` 事件，前端会一直停在“running”。  
我现在保证失败也会补一个 failed trace。

这部分也加了测试，[runtime_tools_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime_tools_test.go:273)。

---

**六、为什么我没有做 interrupt/resume**

因为你后面已经明确了：  
你当前缺参数场景，本质上就是普通多轮对话。

比如：
- 用户说“查我的排名”
- 模型说“你要查 leetcode 还是 luogu”
- 用户下一轮回答“leetcode”

这根本不需要 runtime 级 interrupt。  
这只是 assistant 在正常追问。

所以我这次坚持了一个原则：

**缺参时，模型自然追问；不是 runtime 状态机去接管。**

这让你的 ReAct V1 保持简单，也更符合当前产品阶段。

---

**七、我为什么要加 `aiContextAssembler`、`aiPromptBuilder`、`ChatModelFactory`**

这三个是“为未来留钩子”，但又不污染现在逻辑。

### `aiContextAssembler`
这是给你未来接：
- memory recall
- 历史裁剪
- 压缩摘要

的位置。

如果以后你要做：
- 最近 20 条消息 + 历史摘要
- 召回用户长期偏好
- 对话主题记忆

就改它，不需要把 `AIService` 撕开。

### `aiPromptBuilder`
这是给你未来做：
- 不同业务模式下不同 prompt
- A/B prompt 实验
- prompt 配置外置化

的位置。

### `ChatModelFactory`
这是给你未来做：
- LLM 网关
- provider 路由
- 模型降级
- 熔断重试

的位置。

它只存在于 infrastructure，不会上浮到 service。  
这点很重要，因为这样不会让 `service/system` 被 Eino 反向污染。

---

**八、测试是怎么证明这套实现可用的**

我补了几组关键测试。

### 上下文装配测试
在 [aiContext_test.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext_test.go:54) 和 [aiContext_test.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext_test.go:104)

验证：
- 默认会用 stored history
- 默认 prompt 会包含工具说明和“不要猜测”
- memory/compressor/promptBuilder 注入后会被调用
- 没注入时行为不变

### runtime 纯文本路径
在 [runtime_tools_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime_tools_test.go:91)

验证：
- 无工具时按纯文本流跑
- 事件顺序正确

### runtime 工具路径
在 [runtime_tools_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime_tools_test.go:133)

验证：
- tool schema 绑定成功
- 动态 prompt 注入成功
- tool arguments 原文透传成功
- tool call context 正确带入
- 事件顺序正确

### 缺参自然追问
在 [runtime_tools_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime_tools_test.go:231)

验证：
- 有工具但模型不调用工具，直接追问用户
- 这是合法路径

### 工具失败 trace
在 [runtime_tools_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime_tools_test.go:273)

验证：
- 工具失败时会先发 `tool_call_finished(failed)`
- 不会让前端 trace 卡死在 running

### 自定义模型工厂
在 [chat_model_test.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/chat_model_test.go:10)

验证：
- `ChatModelFactory` 注入时，`NewChatModel` 会优先使用它

---

**九、你现在这套 ReAct 的完整职责边界**

你可以这么记：

### `domain/ai`
只定义协议：
- `Runtime`
- `StreamInput`
- `Tool`
- `Event`

看这里：
[runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/runtime.go:5)  
[tool.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/tool.go:5)  
[event.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/event.go:5)

### `service/system`
只做业务编排：
- 查用户/会话/历史
- 建消息骨架
- 过滤工具
- 组上下文
- 调 runtime
- 做流式收尾

看这里：
[aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:223)  
[aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:55)  
[aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:67)

### `infrastructure/ai/eino`
只做执行：
- 创建 model
- tool binding
- tool loop
- 发事件

看这里：
[runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:79)  
[chat_model.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/chat_model.go:30)

### `aiSink + aiProjector`
只做状态汇聚：
- SSE 输出
- assistant 消息内容折叠
- tool trace 折叠
- DB 落库

看这里：
[aiSink.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSink.go:45)  
[aiProjector.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiProjector.go:104)

---

**十、如果你要继续往下做，下一步最合理是什么**

现在最值得继续做的不是 ADK，也不是 interrupt，而是这三项里的一个：

1. 工具失败分类  
现在 runtime 还是“工具失败即整轮失败”。后面你可以细分：
- 参数错误 -> 更适合让模型追问
- 业务错误 -> 直接解释
- 系统错误 -> 整轮失败

2. 记忆模块落地  
你已经有 `aiMemoryProvider` 入口了。下一步可以直接挂：
- 最近主题摘要
- 用户偏好记忆
- 重要事实召回

3. 上下文压缩  
你已经有 `aiContextCompressor`。可以先做最简单版：
- 历史消息超过 N 条时摘要前半段
- 保留最近 K 条原文

如果你愿意，我下一条可以继续直接给你画一张更具体的图：

**“用户发一句话后，`AIService -> ContextAssembler -> Runtime -> Tool -> Sink -> Projector -> DB/SSE` 的时序图”**

或者我也可以直接继续帮你做下一步实现，比如：
- `memory provider`
- `上下文压缩`
- `工具失败分类`