# 目标

把当前 AI 子域的实现方式和未来扩展点写入正式文档，方便后续开发、联调和评审时有统一说明。

# 范围

- 目标文档：`docs/AI助手架构设计方案.md`
- 说明范围：
  - 当前 AI 请求链路如何从 Router / Controller / Service 进入 Runtime。
  - 当前 `AIRuntime`、`EinoAIRuntime`、`LocalAIRuntime` 的职责边界。
  - 当前工具能力、SSE 事件、Interrupt / Resume、持久化与控制面如何协作。
  - 后续可扩展方向。

# 改动

- 在 `docs/AI助手架构设计方案.md` 中新增一节“当前实现说明与扩展点”。
- 内容以现有代码为准，不引入未实现接口结论。
- 不修改 Go 代码、不修改 OpenAPI、不修改配置。

# 验证

- 只做文档修改，验证方式为人工阅读和路径引用检查。
- 确认新增内容与当前关键代码路径一致：
  - `internal/controller/system/aiCtrl.go`
  - `internal/service/system/aiSvc.go`
  - `internal/service/system/aiRuntime*.go`
  - `internal/infrastructure/ai/eino/*`
  - `internal/repository/system/aiRepo.go`
  - `internal/model/entity/ai.go`

# 风险

- 若后续代码快速演进，文档可能需要同步更新。
- 当前说明会避免过细函数级实现，减少和代码漂移的概率。

# 执行顺序

1. 将本计划从 `pending-` 改为 `approved-`。
2. 编辑 `docs/AI助手架构设计方案.md`。
3. 检查新增章节的标题层级、路径引用和术语一致性。
4. 回报修改摘要。

# 待确认

请确认是否按本计划执行文档修改。
