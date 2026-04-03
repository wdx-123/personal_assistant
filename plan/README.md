# 计划目录说明

## 目录用途

- 本目录用于存放本项目所有待审和已审的执行型计划。
- 只要任务属于新增、重构、修复、联调、排障、迁移、删除、配置调整等会落代码或改规则的工作，必须先在本目录生成计划，再等待确认后执行。
- 纯问答、纯解释、纯代码审查、纯只读排查，不强制生成计划文件。

## 命名规则

- 计划文件路径固定为 `plan/<module>/pending-<task>.md` 或 `plan/<module>/approved-<task>.md`。
- 结构名固定使用英文：根目录为 `plan/`，跨模块目录为 `plan/cross-module/`，状态前缀为 `pending-` 和 `approved-`。
- `<module>` 和 `<task>` 按语义决定中英文：稳定技术名词优先英文，如 `auth`、`permission`；更自然的业务表达可保留中文，如 `组织`、`菜单权限收口`。
- 模块目录按首次使用时创建；当前已预留 `plan/cross-module/` 目录。

## 状态流转规则

- 第一步：先生成待审计划，文件名使用 `pending-<task>.md`。
- 第二步：在对话中回报计划路径和摘要，等待用户明确确认。
- 第三步：确认后，将文件名改为 `approved-<task>.md` 再执行。
- 若执行中范围明显变化，必须新建新的 `pending-<task>.md` 重新审查，禁止静默扩项。

## 计划模板

# 目标

# 范围

# 改动

# 验证

# 风险

# 执行顺序

# 待确认

## 示例路径

- `plan/auth/pending-login-auth-refactor.md`
- `plan/权限/pending-菜单权限收口.md`
- `plan/cross-module/pending-组织权限联调.md`
