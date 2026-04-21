# 目标

基于当前仓库真实代码，新增一篇 AI 架构概览文档，帮助维护者理解 AI 子域的入口、分层、运行时、SSE、落库、工具调用、用户确认和恢复链路。

# 范围

- 新增文档：`docs/ai-architecture-overview.md`
- 只分析 AI 子域，不泛化分析整个项目。
- 覆盖 Router、Controller、Service、Runtime、Sink、SSE、Repository、Entity、Eino 基础设施、控制面与恢复。

# 改动

- 新增 `docs/ai-architecture-overview.md`。
- 文档固定包含：
  - AI 架构总览
  - 核心链路流程
  - 关键目录与文件说明
  - 关键函数说明
  - Mermaid 图
  - 面试时如何介绍这套 AI 架构
- 不修改 Go 代码、配置、OpenAPI 或现有设计文档。

# 验证

- 检查目标文档已创建。
- 检查文档包含 `AICtrl`、`AIService`、`AIRuntime`、`aiStreamSink`、SSE、DB、interrupt、resume 等关键字。
- 本次为纯文档新增，不运行 Go 测试。

# 风险

- 文档依赖当前代码实现，后续 AI runtime 或协议变更时需要同步更新。

# 执行顺序

1. 将本文件改名为 `approved-ai-architecture-overview-doc.md`。
2. 新增 `docs/ai-architecture-overview.md`。
3. 做只读关键字检查。
4. 回报新增文件和验证结果。

# 待确认

用户已明确要求 `PLEASE IMPLEMENT THIS PLAN`，本计划按已确认执行。
