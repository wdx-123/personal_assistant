# 目标

- 明确把本项目“传统 MVC 主体 + AI 子域渐进式 DDD 拆分”的架构定位沉淀到仓库规则与本地 skill 中。
- 补齐 AI 方向的落地约束，避免后续开发再次把 AI 子域误做成“全量 DDD 重构”或“继续堆在 MVC Service 内”的两种极端。
- 让 Codex / 本地协作代理在处理 AI 相关任务时，默认遵守同一套目录边界、依赖方向和演进策略。

# 范围

- `AGENTS.md`
- `.codex/skills/personal-assistant-go-backend/SKILL.md`
- `.codex/skills/personal-assistant-go-backend/references/project-rules.md`
- 可选同步文档：
  - `docs/AI/AI领域+DDD架构拆分.md`

# 改动

- 在 `AGENTS.md` 中补充 AI 子域专项规则，至少覆盖：
  - 项目整体仍以 MVC 为主体，不以“全量 DDD 化”为默认目标。
  - AI 子域允许渐进式引入 `domain/ai` 与 `infrastructure/ai`，但 `controller/router/service/repository` 主体结构继续保留。
  - AI 方向新增能力时，优先判断应放在：
    - `domain/ai`：稳定协议、事件、tool/runtime 抽象。
    - `infrastructure/ai`：Eino / Local / 外部模型 / 第三方 Agent 适配。
    - `service/system`：AI 应用编排，不承载底层框架细节。
  - 禁止把 AI 子域演进误解为“一次性全盘 DDD 重构”。
  - 禁止把 AI runtime / tool / trace / prompt 拼装继续无边界堆进单个 service 文件。
- 在本地 skill 中增加“AI 渐进式 DDD 工作流”说明，至少覆盖：
  - 遇到 AI 子域开发、重构、评审时，先识别本次变更属于协议层、应用编排层还是基础设施适配层。
  - AI 子域优先复用现有 MVC 外壳，只在必要处补 `domain/ai` 与 `infrastructure/ai`。
  - 若用户要求做 AI tool、runtime、事件、trace、恢复、approval 等能力，应优先检查是否符合当前“渐进式 DDD”边界，而不是直接新增散落结构。
  - 评审 AI 代码时，将“依赖方向错误、协议与实现耦合、service 越界膨胀”列为重点问题。
- 在 `references/project-rules.md` 中补充与 AGENTS 对齐的权威表述，避免 skill 与仓库规则脱节。
- 如有必要，在 `docs/AI/AI领域+DDD架构拆分.md` 增补“当前项目口径”段落，明确：
  - 这是 AI 子域的局部 DDD，不是项目整体目录全面改名。
  - A2UI、interrupt、tool、runtimecontrol 等能力可按阶段收缩或重建，但不影响“渐进式 DDD”主口径。

# 验证

- 检查 `AGENTS.md`、skill 和 reference 三处表述是否一致，不出现互相冲突的架构口径。
- 检查新增规则是否能直接指导后续 AI 任务落点判断，而不是停留在空泛概念。
- 检查 skill 中是否明确区分：
  - MVC 主体不动
  - AI 子域局部拆分
  - domain / infrastructure 的边界
- 如同步文档，确认文档措辞与仓库规则一致，不出现“已经全量 DDD 重构”的误导描述。

# 风险

- 若规则写得过泛，后续代理仍可能把 AI 改动继续堆进 `service/system`。
- 若规则写得过硬，可能误导为“所有 AI 代码都必须立即迁入完整 DDD 目录”，反而提高改造成本。
- 若只改 skill 不改 AGENTS / reference，规则来源会分裂，后续执行口径仍不统一。

# 执行顺序

1. 盘点现有 AGENTS、skill、reference 与 AI 架构文档中的表述差异。
2. 设计统一口径：MVC 主体、AI 渐进式 DDD、边界与禁止模式。
3. 更新 `AGENTS.md`。
4. 更新本地 skill 与 `references/project-rules.md`。
5. 视需要同步 `docs/AI/AI领域+DDD架构拆分.md`。
6. 做文本一致性复核并向用户汇总。

# 待确认

- 是否要求同时修改仓库根 `AGENTS.md` 与本地 skill；默认建议两者都改，避免规则分裂。
- 是否把 `docs/AI/AI领域+DDD架构拆分.md` 一并同步为正式口径；默认建议同步一小段，不大改全文。
- 是否需要在 skill 中单独新增“AI 子域任务专用决策树”小节；默认建议新增。
