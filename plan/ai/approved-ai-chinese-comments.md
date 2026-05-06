# 目标

为本次 AI 子域最小流式对话重构新增的代码补充中文注释，风格对齐 `AICtrl.StreamConversation` 这类“职责、参数、返回值、核心流程、注意事项”的说明。

# 范围

- `internal/domain/ai/*`
- `internal/infrastructure/ai/local/*`
- `internal/infrastructure/ai/eino/*`
- `internal/core/ai.go`
- 如有必要，补充 `internal/service/system/aiProjector.go` 中新增最小投影逻辑的中文注释。

# 改动

- 只补充注释，不改业务逻辑、接口签名、配置、路由、DTO 或数据库结构。
- 注释优先覆盖导出的类型、导出函数、核心私有函数和关键流程。
- 避免无意义注释；只解释职责、输入输出、执行流程和边界约束。

# 验证

- 执行 `gofmt`。
- 执行 `go test ./internal/domain/ai/...`。
- 执行 `go test ./internal/infrastructure/ai/...`。
- 执行 `go test ./internal/service/system/...`。

# 风险

- 仅注释变更，业务风险低。
- 需要避免注释和实际行为不一致。

# 执行顺序

1. 审核新增文件中缺少中文注释的位置。
2. 逐文件补充中文注释。
3. 格式化并运行相关测试。

# 待确认

等待用户确认后，将本计划改名为 `approved-ai-chinese-comments.md` 并实施。
