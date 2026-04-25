可以，按“启动链路 -> 连接逻辑 -> collection 初始化 -> 为什么这么设计”给你拆开讲。

**整体链路**
真正触发连接 Qdrant 的入口不在 `qdrant.go` 本身，而是在 [init.go](d:/workspace_go/test/go/personal_assistant/internal/init/init.go:25)。

启动顺序里，代码先做这几步：

1. `godotenv.Load()` 读本地 `.env`
2. `core.InitConfig("configs")` 把 `configs.yaml + 环境变量` 装进 `global.Config`
3. `global.Log = core.InitLogger()`
4. `core.InitQdrant(context.Background())`
5. 成功后把返回的 client 放进 [global.go](d:/workspace_go/test/go/personal_assistant/global/global.go:21) 里的 `global.QdrantClient`

对应位置是 [init.go:35](d:/workspace_go/test/go/personal_assistant/internal/init/init.go:35) 到 [init.go:40](d:/workspace_go/test/go/personal_assistant/internal/init/init.go:40)。

也就是说，Qdrant 是“启动期基础设施”，不是业务请求到来时临时连。

**配置是怎么进入这里的**
Qdrant 的配置结构在 [internal/model/config/qdrant.go](d:/workspace_go/test/go/personal_assistant/internal/model/config/qdrant.go:3)。

核心字段分成两类：

- 连接参数：`Enabled`、`Endpoint`、`GRPCHost`、`GRPCPort`、`APIKey`、`UseTLS`
- collection 参数：`CollectionName`、`VectorSize`、`Distance`、`InitCollection`、`TimeoutSeconds`

这些字段在 `config.go` 里从 viper 读取，再挂到 `global.Config.Qdrant`。所以 `InitQdrant()` 里第一件事就是拿 [qdrant.go:38](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:38) 的 `global.Config.Qdrant`。

**连接 Qdrant 的核心逻辑**
主入口是 [internal/core/qdrant.go:34](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:34) 的 `InitQdrant(ctx)`。

它的执行顺序是：

1. 先检查 `global.Config` 是否为空  
   位置：[qdrant.go:35](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:35)

2. 读取 `qdrantCfg := global.Config.Qdrant`  
   位置：[qdrant.go:38](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:38)

3. 如果 `qdrant.enabled=false`，直接跳过  
   位置：[qdrant.go:39](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:39)  
   这里返回 `nil, nil`，表示“不启用，不报错”。

4. 调 `newQdrantClient(qdrantCfg)` 创建官方 Go client  
   位置：[qdrant.go:44](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:44)

5. 创建一个带超时的上下文  
   位置：[qdrant.go:49](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:49)  
   超时时间来自 `qdrant.timeout_seconds`，如果没配或非法，就兜底 10 秒。

6. 对 Qdrant 做 `HealthCheck()`  
   位置：[qdrant.go:52](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:52)  
   这一层不是“真的做业务”，只是确认：
   - 地址对不对
   - 端口通不通
   - API key 是否可用
   - TLS 配置是否正确
   - 服务是不是活着

7. 如果 `qdrant.init_collection=true`，继续做 collection 初始化/校验  
   位置：[qdrant.go:59](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:59)

8. 全部通过后，把 client 返回给初始化编排层，最终挂到 `global.QdrantClient`

**为什么我不用 6333，而是 6334**
这是这段逻辑里最容易混淆的点。

- `6333` 是 Qdrant 的 HTTP/REST 端口
- `6334` 是 Qdrant 的 gRPC 端口

你这里用的是官方 Go SDK：`github.com/qdrant/go-client/qdrant`。这个 SDK 的高层 client 走的是 gRPC，所以真正建连时必须用 `grpc_host + grpc_port`，默认就是 `6334`。

对应代码在 [qdrant.go:74](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:74)：

- 先算出 host
- 再决定 port
- 然后调用 `qdrant.NewClient(...)`

真正传进去的是：

- `Host`
- `Port`
- `APIKey`
- `UseTLS`

不是 `Endpoint`

`Endpoint` 在这里的作用只是“兼容上一阶段配置”，当你没显式写 `grpc_host` 时，我会从 `endpoint` 里把 host 解析出来。这个逻辑在 [qdrant.go:167](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:167)。

优先级是：

1. `GRPCHost` 有值，就直接用它
2. `GRPCHost` 为空，就从 `Endpoint` 里解析 host
3. 两者都没有，就报错

也就是说：

- `QDRANT_ENDPOINT=http://180.184.87.86:6333`
- `QDRANT_GRPC_HOST=180.184.87.86`
- `QDRANT_GRPC_PORT=6334`

这三者现在是并存的，但真正给 Go client 用的是后两者。

**collection 初始化在干什么**
这一段在 [qdrant.go:105](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:105) 的 `ensureQdrantCollection(...)`。

它不是“往向量库写数据”，它做的是“确保将来可以安全写”。

执行顺序是：

1. 校验本地配置是否合法  
   位置：[qdrant.go:110](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:110)
   - `CollectionName` 不能为空
   - `VectorSize` 必须 > 0
   - `Distance` 要能转换成 Qdrant 枚举

2. 调 `CollectionExists()` 看 collection 在不在  
   位置：[qdrant.go:122](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:122)

3. 如果不存在，就创建  
   位置：[qdrant.go:126](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:126)

创建时用的是：

- `collection_name = ai_knowledge_chunks`
- `size = 1024`
- `distance = cosine`

也就是：

```go
VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
    Size:     1024,
    Distance: qdrant.Distance_Cosine,
})
```

4. 如果已经存在，就不是直接跳过，而是继续做 schema 校验  
   位置：[qdrant.go:140](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:140)

它会读取已有 collection 的配置，然后检查：

- 是不是单向量 collection
- 维度是不是 1024
- 距离算法是不是 cosine

如果不一致，直接报错，不让服务启动。

**为什么要校验而不是默认复用**
因为向量库最怕“表存在，但 schema 不对”。

比如你以后 embedding 模型吐的是 `1024` 维，但线上 collection 是 `1536` 维，后果不是“效果差一点”，而是后续写入和检索直接出问题。

所以我这里的策略是：

- 不存在：创建
- 已存在且匹配：通过
- 已存在但不匹配：启动失败

这是典型的 fail-fast。

**`parseQdrantDistance()` 干了什么**
位置：[qdrant.go:188](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:188)

因为配置文件里写字符串更自然：

- `cosine`
- `dot`
- `euclid`
- `manhattan`

但 SDK 需要的是枚举：

- `qdrant.Distance_Cosine`
- `qdrant.Distance_Dot`
- ...

所以这里做一次转换。这样配置层保持可读，SDK 细节被关在 core 层，不污染业务层。

**超时逻辑**
位置：[qdrant.go:206](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:206)

`qdrantTimeout()` 的作用很简单：

- 如果你配了 `timeout_seconds`，就用配置值
- 如果没配或 <=0，就用 10 秒默认值

然后 [qdrant.go:49](d:/workspace_go/test/go/personal_assistant/internal/core/qdrant.go:49) 用它包一个 `context.WithTimeout(...)`

这样启动时不会因为 Qdrant 网络抖动一直卡死。

**一句话总结这套逻辑**
这套连接逻辑本质上是：

“启动时根据配置创建官方 gRPC client -> 做健康检查 -> 自动创建或严格校验基础 collection -> 成功后挂到 `global.QdrantClient` 供后续业务使用；任何关键步骤失败都直接阻断启动。”

如果你愿意，我下一步可以继续给你讲两件事：

1. 这套逻辑现在有哪些风险点/可以继续优化的地方  
2. 以后真正做“写入向量”和“检索”时，应该怎么接在这套初始化后面