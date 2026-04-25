# 目标

- 收口 `internal/service/system/aiTool.go` 中与工具元数据和分组映射相关的重复真相，降低 `switch spec.Name` 带来的维护成本。
- 在不改变现有工具协议、权限策略、运行时行为和渐进式选择结果的前提下，拆分 `aiTool.go` 的职责边界。
- 让新增 AI tool 时优先在工具定义处一次性声明 `group / brief / policy / spec / validate / call`，避免后置补元数据。

# 范围

- `internal/service/system/aiTool.go`
- `internal/service/system/aiTool_test.go`
- 与 AI tool 渐进式元数据装配直接相关的 `internal/service/system/aiProgressive*.go` 测试或辅助代码

# 改动

- 将 AI tool 的分组和 brief 元数据从“按 `spec.Name` 二次推导”改为“在工具注册时直接声明”。
- 拆分 `aiTool.go` 内当前混合的职责，至少把以下内容从单文件中收口：
  - registry / visible filtering
  - tool metadata / group metadata
  - prompt helper
  - 具体 tool 定义
- 保留现有 `policy.Kind` 等有限枚举分发；不强行消灭所有 `switch`，只清理重复映射和单文件堆积。
- 删除或缩减 `aiBuildToolMetadata` 这类基于工具名的后置映射逻辑，避免工具定义和元数据定义分离。
- 对组级说明和工具 brief 改成更稳定的声明式结构，保证渐进式 selector 继续消费同样的数据形状。
- 补齐或调整测试，覆盖：
  - 可见工具元数据仍能正确分组
  - 按组展开与按名称展开行为不变
  - 未引入新的权限可见性回归

# 验证

- 运行 AI tool 相关单测，至少覆盖：
  - `go test ./internal/service/system -run TestAIToolRegistry`
  - 必要时补充 `aiProgressive` 相关测试
- 若结构拆分涉及编译依赖，执行最小必要的包级编译或测试，确认无循环依赖和无行为回归。

# 风险

- 元数据迁移过程中若有遗漏，可能导致某些 tool 在 progressive selector 中分组错误或 brief 缺失。
- 若拆分文件时误改初始化顺序，可能影响 registry 构建结果。
- 当前 `aiTool.go` 中工具实现较多，若一次拆分过大，review 成本会升高；需要控制为“结构收口，不扩功能”。

# 执行顺序

1. 盘点 `aiTool.go` 中 registry、metadata、prompt、tool implementation 四类职责的边界。
2. 设计新的声明式元数据承载方式，确保每个 tool 在定义时就带齐 metadata。
3. 拆出 registry / metadata / prompt 辅助代码，缩减主文件体积。
4. 迁移具体 tool 的 metadata 装配逻辑，删除后置 `switch spec.Name` 映射。
5. 运行并补齐相关测试，确认行为保持一致。

# 待确认

- 默认本次只做结构重构，不调整 tool 名称、参数协议、权限模型和 selector 策略。
- 默认不把 AI tool 整体下沉到 `internal/domain/ai`，继续保持 `service/system` 作为应用编排与工具注册入口。
