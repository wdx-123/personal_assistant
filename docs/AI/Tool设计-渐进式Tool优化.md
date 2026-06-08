整体上，我这次是把“工具发现”和“工具执行”拆开了。  
以前是：`可见工具 -> 全量详细暴露 -> 直接 ReAct`。  
现在是：`可见工具 -> 先选组 -> 再选工具 -> 只把最终工具交给 ReAct`。

你可以按这 4 层理解。

**1. 目录和职责**
`domain/ai` 现在多了一组“渐进式选择协议”，在 [progressive.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/progressive.go:1)。这里定义的不是具体实现，而是共享类型：

- `ToolGroupID`
- `ToolBrief`
- `ToolGroupBrief`
- `ToolGroupSelectionInput`
- `ToolGroupSelection`
- `ToolSelectionInput`
- `ToolSelection`

这层的作用是：让 `service/system` 和 `infrastructure/ai/eino` 可以通过稳定协议通信，而不是互相依赖实现细节。

`service/system` 现在负责“业务编排 + 工具发现”：
- 主入口还是 [aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:231)
- 渐进式选择编排在 [aiProgressive.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiProgressive.go:1)
- 工具注册、分组、brief 生成、展开逻辑在 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:67)
- prompt 构造接口在 [aiPrompt.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiPrompt.go)
- 上下文装配在 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:1)

`infrastructure/ai/eino` 现在负责“内部 selector + 最终 runtime 执行”：
- selector 在 [selector.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/selector.go:1)
- 最终 ReAct runtime 还是 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:79)

`service/system/supplier.go` 是装配层，在 [supplier.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/supplier.go:26)。  
这里负责把 `ProgressiveToolSelector` 注入进 `AIService`，但 `AIService` 自己不 import Eino。

**2. 现在一次请求怎么跑**
主入口还是 `AIService.StreamConversation(...)`，在 [aiSvc.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiSvc.go:231)。

它现在的核心流程是：

1. 查会话、查用户、查历史消息
2. 创建 user/assistant 消息骨架并落库
3. 构造 `ToolCallContext`
4. 调 `filterVisibleAITools(...)` 得到本轮可见工具
5. 调 `contextAssembler.Build(...)` 只生成基础 `History`
6. 调 `buildAIToolExecutionPlan(...)` 做三段式工具选择
7. 把 `executionPlan.Tools + executionPlan.DynamicSystemPrompt` 交给最终 `runtime.Stream(...)`

所以现在 `runtime` 不再负责“我有哪些工具可选”，它只负责“拿着已经选定的工具去跑最终 ReAct”。

**3. 三段式选择是怎么实现的**
核心函数是 [aiProgressive.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiProgressive.go:27) 的：

```go
func (s *AIService) buildAIToolExecutionPlan(
    ctx context.Context,
    query string,
    history []aidomain.Message,
    visibleTools []aidomain.Tool,
    principal aidomain.AIToolPrincipal,
) (aiToolExecutionPlan, error)
```

它的 5 个参数分别代表：
- `ctx`：内部选择器调用上下文
- `query`：当前用户问题
- `history`：已经装配好的历史消息
- `visibleTools`：本轮经过权限过滤后真正可见的工具
- `principal`：当前用户最小授权事实，用来生成最终 prompt

它返回 `aiToolExecutionPlan`，这个结构只有两个字段，在 [aiProgressive.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiProgressive.go:15)：
- `Tools []aidomain.Tool`
- `DynamicSystemPrompt string`

也就是说，这个函数的职责非常聚焦：  
**决定最终到底给 runtime 哪些工具，以及给它什么 prompt。**

内部流程是：

1. 先做一个 `defaultPlan`
   - `Tools = visibleTools`
   - `DynamicSystemPrompt = 全量可见工具的最终 prompt`
   这是回退路径。

2. 如果没工具、没 selector、没 registry
   - 直接返回 `defaultPlan`

3. 调第一阶段 selector：`SelectGroup(...)`
   输入是 [progressive.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/progressive.go:50) 的 `ToolGroupSelectionInput`
   - `Query`
   - `History`
   - `Groups`

4. 第一阶段结果有 3 种：
   - `direct_answer`
   - `ask_user`
   - `select_group`

5. 如果是 `direct_answer` 或 `ask_user`
   - 不注入任何工具
   - 只构造一个 decision prompt
   - 最终还是走一次普通 `runtime.Stream(...)`
   这里用的是 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:593) 的 `buildAIDecisionPrompt(...)`

6. 如果是 `select_group`
   - 先找到对应 `ToolGroupBrief`
   - 再从这个组里取 `ToolBrief`
   - 调第二阶段 selector：`SelectTools(...)`

7. 第二阶段输出 `ToolSelection`
   在 [progressive.go](/D:/workspace_go/test/go/personal_assistant/internal/domain/ai/progressive.go:73)
   - `SelectedToolNames`
   - `Confidence`
   - `Reason`

8. 如果 `confidence=high`
   - 用 `ExpandVisibleToolsByNames(...)` 精确展开
9. 如果 `confidence=low` 或展开为空
   - 用 `ExpandVisibleToolsByGroup(...)` 预扩为整组工具

10. 最后用选中的工具构造最终 prompt，然后进入真正 ReAct

**4. 工具 registry 现在多了什么**
`AIDeps` 在 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:67) 多了一个：

- `ToolSelector aiProgressiveToolSelector`

`aiServiceTool` 在 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:107) 多了两个字段：

- `group aidomain.ToolGroupID`
- `brief aidomain.ToolBrief`

注意这里我没有把 12 个 tool constructor 全部改成手写 metadata，而是统一在 `newAIToolRegistry(...)` 之后调用 `decorateCatalogMetadata()`，见 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:164)。  
这样做的好处是：改动集中，风险小。

新增的几个 registry 方法是这次重构的关键：

- [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:294) `ListVisibleToolGroupBriefs(...)`
  作用：把本轮可见工具压缩成组级 brief，给第一阶段 selector 用

- [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:311) `ListVisibleToolBriefsByGroup(...)`
  作用：从已选组里取组内工具 brief，给第二阶段 selector 用

- [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:326) `ExpandVisibleToolsByNames(...)`
  作用：把第二阶段选出来的工具名，恢复成真正的 `aidomain.Tool`
  这里我限制最多 3 个

- [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:360) `ExpandVisibleToolsByGroup(...)`
  作用：低置信时直接回退成整组工具

metadata 本身是通过 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:431) 的 `aiBuildToolMetadata(...)` 生成的。  
它会根据 `ToolSpec.Name` 给每个工具补：
- 所属组
- `summary`
- `when_to_use`
- `required_slots`
- `domain_tags`

**5. prompt 现在怎么分工**
以前 `aiContextAssembler.Build(...)` 会顺手把动态 prompt 也做掉。  
现在不会了。

`aiContextSnapshot` 在 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:49) 现在只剩：
- `History []aidomain.Message`

也就是说，`contextAssembler` 现在只负责“基础上下文”，见 [aiContext.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiContext.go:72)。

prompt 构造现在分成两类，在 [aiPrompt.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiPrompt.go)：
- `BuildDynamicPrompt(tools, principal)`
  用于最终进入 ReAct 时
- `BuildDecisionPrompt(decision, reason, missingSlots)`
  用于第一阶段直接判定“回答”或“追问”的场景

最终工具 prompt 用的是 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:565) 的 `buildAIToolDynamicPrompt(...)`。  
这版已经被我压短了，现在只保留：
- 本轮只能用当前注入工具
- 缺关键标识别猜
- recoverable error 怎么处理
- RFC3339 规则
- 当前组织上下文
- 工具列表仅保留 `name + description`

不再逐个展开所有参数细节。

而第一阶段直接答复/追问时，用的是 [aiTool.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/aiTool.go:593) 的 `buildAIDecisionPrompt(...)`。

**6. Eino selector 是怎么实现的**
真正的 selector 在 [selector.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/selector.go:15)：

```go
type ProgressiveToolSelector struct {
    model        einomodel.BaseChatModel
    systemPrompt string
}
```

这说明它本质上就是一个很薄的“内部模型调用器”，并不是新 runtime。

构造函数是 [selector.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/selector.go:20)：

```go
func NewProgressiveToolSelector(ctx context.Context, opt Options) (*ProgressiveToolSelector, error)
```

参数 `opt` 复用你现有的 `eino.Options`，所以：
- provider
- model
- api key
- base url
- system prompt
这些都不用重新定义。

两个核心函数：

- [selector.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/selector.go:35) `SelectGroup(...)`
- [selector.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/selector.go:61) `SelectTools(...)`

它们都不是流式，也不绑定 tools，而是内部调用 `generateJSON(...)`，见 [selector.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/selector.go:92)。

`generateJSON(...)` 的逻辑很简单：
1. 组装 `system + selector prompt + history + query`
2. 调 `model.Generate(...)`
3. 只接受 JSON
4. 用 `unmarshalSelectorJSON(...)` 做解析和清洗

这里有一个很重要的设计点：  
**第一、二阶段 selector 只消费 brief，不消费 full schema。**

所以它的 prompt 里只会出现：
- group brief JSON
- tool brief JSON

不会出现：
- `format=rfc3339`
- `enum`
- `min/max`
- `examples`
这些 full schema 细节。

**7. selector 是怎么注入进服务层的**
装配在 [supplier.go](/D:/workspace_go/test/go/personal_assistant/internal/service/system/supplier.go:26)。

这里的逻辑是：
1. 如果当前 `AIRuntimeMode=eino`
2. 用同一套 AI 配置创建 `NewProgressiveToolSelector(...)`
3. 成功则把它塞进 `AIDeps.ToolSelector`
4. 失败则只打 warn，回退到单阶段路径

所以 service 层只知道一个接口 `aiProgressiveToolSelector`，并不知道 Eino 的具体类型。  
这就是这次我刻意保住的边界。

**8. 最终 runtime 有没有被改坏**
没有，本质上没变。

最终执行仍然走 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:79) 的 `Stream(...)`，有工具时仍然走 [runtime.go](/D:/workspace_go/test/go/personal_assistant/internal/infrastructure/ai/eino/runtime.go:164) 的 `streamWithTools(...)`。

区别只在于：

- 以前 `input.Tools = 所有可见工具`
- 现在 `input.Tools = 渐进式选择后的工具`

所以现有的：
- schema 校验
- 参数自修正
- tool observation
- tool trace
- SSE 事件
都还能继续用。

**9. 你可以怎么记住这套实现**
一句话版本：

- `domain/ai/progressive.go`
  定义“怎么选工具”的协议
- `service/system/aiTool.go`
  把完整工具变成 `group + brief`
- `service/system/aiProgressive.go`
  做三段式执行计划
- `infrastructure/ai/eino/selector.go`
  用模型做两阶段内部选择
- `service/system/aiSvc.go`
  把最终选中的工具交给原来的 ReAct runtime

如果你愿意，我下一条可以继续给你画一张非常具体的时序图：  
`用户提问 -> AIService -> ToolRegistry -> GroupSelector -> ToolSelector -> Runtime -> Tool -> Sink/Projector`。