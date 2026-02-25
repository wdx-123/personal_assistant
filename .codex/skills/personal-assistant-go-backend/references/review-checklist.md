# 后端评审清单

评审本仓库后端改动时，按以下清单执行。

## A. 功能正确性

- 确认行为符合需求与现有 API 语义。
- 确认请求校验与边界条件处理完整。
- 对分页、过滤、空结果等场景进行显式确认。

## B. 分层合规

- Controller 不承载业务编排。
- Service 不直连 DB。
- Repository 是唯一执行 DB CRUD/JOIN 的层。
- Controller 不直接调用 Repository。

## C. DTO 与响应一致性

- API 入参/出参使用 DTO，而非暴露 Entity。
- 使用 `pkg/response` 统一返回。
- 失败响应走统一错误路径。

## D. 错误链路完整性

- Repository 返回原始 `error`，不耦合业务语义。
- Service 使用 `pkg/errors` 包装业务错误。
- Controller 统一 `global.Log.Error` 记录并返回 BizError 响应。
- 错误码分段符合 `pkg/errors/codes.go`。

## E. Context 与长链路调用

- DB 与长链路调用均传 `context.Context`。
- Service 与 Repository 边界保持 context 传递。

## F. 配置与硬编码

- 可变运行参数已外置到配置结构体。
- 业务代码通过 `global.Config` 读取配置。
- Service 逻辑中无环境相关硬编码常量。

## G. 安全与访问控制

- 需要鉴权的路由挂载在正确分组。
- 需要权限校验的路由包含权限中间件路径。
- Controller 中用户身份提取按规范使用 `pkg/jwt.GetUserID(c)`。

## H. 测试与验证

- 对行为变化补充或更新测试（可行时）。
- 合并前至少执行相关 build/test 命令。
- 若无法完成某些验证，明确剩余风险。

## I. 输出格式

输出评审结果时：

1. 先列问题项，并按严重级别排序。
2. 每个问题附文件和行号。
3. 有不确定点时补充问题或假设。
4. 概览总结简短且放在后面。
