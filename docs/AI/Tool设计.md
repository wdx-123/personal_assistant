@[TOC](如何设计一个灵活、高效、安全的 AI 工具系统)

在 AI 应用里面，决定系统上限的不止有LLM，还包括工具集(`tools`)的管理，也就是（Function Calling）。
在没有 `tools` 之前，大模型也`只能`文本对话，无法进行具体的业务操作。而 `tools` 出来之后，就相当于为大模型安装上了双手。让他可以接触`真实的业务环境`。

很多人，对工具调用的固有认知还停留在，我设计一个 JSON Schema - 告诉大模型 `调用` 这个函数需要`传入的信息`，然后大模型调用工具并**产生数据**。

但在生产环境中，这是只是最基础的！

因为在开发中，很快会遇到很多问题：

1、**协议是否是统一** <small>（比如东边用Json Schma，西边用自定义类型）
2、**权限边界是否清晰**<small>（保证LLM不会越过权限做事)</small>
3、**运行过程是否可控** <small>(是否可追踪，能让用户看到，在干嘛)</small>

这些都是需要考虑的问题，而这`三点`，也是我下文着重突出的内容！





## 技术设计：
这里我宏观讲解一下，`实现`与`使用`。

### 宏观定义：

设计工具系统首先要考虑到的，就是`协议统一`问题！
目的就是为了不让把`工具`写的散乱，所以定义一套约束，这也就是接口的意义。
如，你需要知道：
1. 这个函数 **叫什么**名字，**能干嘛**，**吃什么参数**（ToolSpec）
2. 他如何**调用**，目标的tool（TooCall）
3. 你调用过工具后，**返回的参数**也需要统一（ToolResult）
4. 同时为了统一性，你需要把`定义` 与 `执行` 绑到一个协议上。

具体函数如下：
```go
type ToolParameterType string

const (
	ToolParameterTypeObject  ToolParameterType = "object"
	ToolParameterTypeString  ToolParameterType = "string"
	ToolParameterTypeInteger ToolParameterType = "integer"
	ToolParameterTypeNumber  ToolParameterType = "number"
	ToolParameterTypeBoolean ToolParameterType = "boolean"
	ToolParameterTypeArray   ToolParameterType = "array"
)

type ToolParameter struct {
	Name        string
	Type        ToolParameterType
	Description string
	Required    bool
	Enum        []string
	Properties  []ToolParameter
	Items       *ToolParameter
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  []ToolParameter
}

type ToolCall struct {
	ID            string
	Name          string
	ArgumentsJSON string
}

type ToolResult struct {
	Output         string
	Summary        string
	DetailMarkdown string
}

type Tool interface {
	Spec() ToolSpec
	Call(ctx context.Context, call ToolCall, callCtx ToolCallContext) (ToolResult, error)
}

```


### 技术选择
我对入参的描述，是仿照的 Json Schema模式，而并没有直接选择，因为它对目前的我来说有点重，并且可读性也不高。
重点是，目前我定义的这套接口中，重心不在这。
因为工具系统的设计，
还需要着重考虑：
1、**权限的治理**（哪些工具是有权限调用的）
2、**可见性过滤**（哪些工具适合给大模型调用，不适合的就不包装发给大模型了）
3、调用的内容如何**被观测到**。

所以，虽 Json Schema 的兼容性会更高些，但是最差的结果，也就是我写一层兼容而已。因为我目前的中心明显不在这里。




## 架构设计

如果要从架构方面说起，最重要的就是`业务实现`与`技术实现`了。

1. **业务实现**，回答的是‘这轮 AI 到底该做什么’，比如当前 用户是谁、有没有权限、哪些工具本轮可见等
2. **技术实现**：‘我用什么手段把它做出来’，比如用 Eino 做 tool calling、用 GORM 落库等

为了这两种，我将这个模块拆分成了 `service` 、`domain`、`infra` 三层。
其中 `domain` 做领域层，service与infra共同遵循。

然后service层就是做些业务编排，比如先通过用户权限，筛选出可以工具，然后拼装prompt提示词。最后统一调用AI。
而Infrastructure层，做的是：用Eino对工具进行处理调用，解决这次调用问题！

这样做的好处是，权限变了，工具变了，我只用改动service层，
但是如果我像换AI的编排框架，直接换infra层就行，也不会对其他造成影响。

另外就是为了维护系统的，灵活与高效。
我`没有`让工具系统 `直接依赖` 完整的业务服务。而是分别调用了几个不同的业务接口，如`可观测性模块`接口、`权限模块`接口..
避免了`上帝接口`的出现。也算是遵守了**单一职责原则**。
这样工具层`只依赖`他所需要的真正能力，而且后续也方便Mock接口，做单元测试等...






## 安全性设计
### 灵活权限设计：
在安全维度的设计，我最核心的点，就是`没有`对身份进行`硬编码`。
我最初会在service层，获取三种东西：user_id、org_id、以及他是否是管理员。
从而为他分配三种权限：**self_only**、**OrgCapability**、**SuperAdminOnly**。
其中self_only，用来告诉它，它是由基础权限的。
然后OrgCapability，用来过滤他在组织中有哪些权限做事。
superAdminOnly，代表它可以进行运维管理。

### 安全兜底：
最后通过 capabilities，进行第一次权限过滤，并进入。
当然，是获取工具列表前，用权限过滤一次。
然后大模型调用 tool 时，在对权限进行筛选一次。




## 扩展性与可维护性设计：
### 新增工具是低成本的
因为domain层，已经将协议定义好。而之后要做的只是`增量开发`。
所以之后新增，仅需
- 声明一下工具的`元数据`
- 所属访问策略
- 以及工具实现的具体逻辑。
而不需要把新增的工具放到一个大大的switch中！

### 修改权限也更加灵活方便
修改权限的成本更低，比如`想为你分配`这个`工具`的`权限`，只需要你的角色分配对应`资源` capability 即可。
新增的话，也只需要新增一个 capability 能力，然后分配给对应的**角色**，
注册工具时，就会自动筛选掉，不需要再操心其他。



## 性能与效率
最典型的就是两点。
1、分路执行，如果你什么权限都没有，也就代表你使用不了tool，所以可以直接走纯文本模式。而不必走ReAct。
2、只暴露本轮可见工具。系统不会把全量的tools直接扔给大模型，即减少了token消耗，有减少了，因为越权问题，而导致的访问失败与风险。




## 用户体验
对于工具系统而言，`无法`直接对前端UI造成大的改动与美化。
但是我可以通过 `tool_call_started` 与 `tool_call_finished`。
来让前端知晓，执行到了那一步了，执行信息是什么。
让用户对生产出来的效果，有一个基础认知。这远比AI好几秒，突然给出一个结果好的多。

## 总结
如果只看原始的 Function Calling ，只能在demo场景应用。
在生产环境中，有三个核心问题需要注意，分别是**协议定义是否统一**、**权限是否能得到保证**、**执行过程是否可以被观测**。
为了解决这三点，我从三方面进行入手。

**第一，我自定义了轻量级统一工具协议**，我未用Json Schema，而只是模仿，因为Json Schema对我而言比较重，并且对人来说可读性也不是很高。
同时在我工具集的设计中，我的重心是放在权限与可观察性上的。
不过这并不代表我没有在意，在后期非要加上的话，我可以多加一层适配层。

**第二，三层架构的拆分**。我将我的工具集拆分成了三层。
分别为Service，专注于业务实现，他的作用就是将各个业务函数编排到一起，去实现对应的功能。
其次为domain层，定义好协议，如工具的元数据（出参、入参、工具名称..）的接口，方便后续做增量开发。
最后为infra，这层是基础设施。专注于技术实现，比如我采用的具体技术是Eino，说不定以后会换成其他AI框架，像Langchain啊，都不是不行。

**第三， 双重安全防护措施**，在注册一次工具调用的时候，我会先根据权限筛选一遍，这样不仅可以省token，而且可以防止越界。在LLM调用具体tool的时候，我会在判断一遍。

故，因这三点，我做到了标准化、安全性、解耦、性能！
