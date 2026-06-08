# 目标

重写项目根目录 `README.md`，将其调整为展示 + 面试导向的项目说明，覆盖项目定位、核心能力、架构主线、启动依赖、接口分组和文档入口。

# 范围

- 修改 `README.md`。
- 本计划文件按规则流转为 `approved-readme-rewrite.md` 后执行。
- 不修改业务代码、配置文件、接口文档或 CI 配置。

# 改动

- README 首页结构调整为：
  - 项目定位
  - 核心能力
  - 架构总览
  - 重点实现链路
  - 技术栈
  - 本地启动
  - Docker 启动
  - CLI/CI
  - 接口分组
  - 文档索引
  - 安全说明
- 项目表述采用保守口径：
  - `传统 MVC 主体 + AI 子域渐进式 DDD`
  - 不宣称全量 DDD 架构
  - 明确 `internal/domain/ai` 与 `internal/infrastructure/ai` 的职责边界
- 启动说明按当前代码修正：
  - Go 1.24 / toolchain 1.24.9
  - MySQL、Redis 为基础依赖
  - Qdrant 默认启用，未配置时可关闭
  - Docker Compose 只启动 app，不包含 MySQL / Redis / Qdrant

# 验证

- 人工检查 Markdown 结构。
- 执行 `go test ./...`，确认文档改动未伴随业务代码破坏。

# 风险

- README 中如过度描述未默认开启的能力，容易在面试追问中被质疑；因此 AI memory、Qdrant 等能力按“配置驱动/可启用”表述。
- 当前配置模板包含较多占位项，README 只列关键必填项，避免变成配置手册。

# 执行顺序

1. 将本文件改名为 `approved-readme-rewrite.md`。
2. 替换 `README.md`。
3. 检查 Markdown 内容。
4. 运行 `go test ./...`。

# 待确认

用户已明确要求执行该计划。
