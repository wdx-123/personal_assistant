# Casbin + RBAC 多租户权限系统架构文档

## 1. 文档定位

本文档描述 `personal_assistant` 仓库中权限系统的**当前代码实现**（含多租户组织维度），用于：

1. 新成员快速理解权限链路。
2. 后端改造时进行回归核对。
3. 联调时定位“为什么这个用户在这个组织下能/不能访问某 API”。

> 适用范围：`internal/*`、`pkg/casbin`、`configs/model.conf`、`flag/flagSql.go`。

---

```
新增设计层次：
一、前端层：展示/交互控制
二、API 层：入口访问控制
三、Service / action 层：资源操作授权

对应着：
   UI 展示层权限
   API 入口层权限
   业务动作层权限
```


## 2. 设计总览

### 2.1 核心模型

本项目的权限语义是：

1. 用户身份带组织：`subject = userID@orgID`
2. 角色是全局模板：`role.code`（不带 org）
3. API 资源标识：`path:method`
4. 菜单资源标识：`menu.code`

对应权限链路：

1. 用户-组织-角色：`user_org_roles`
2. 角色-菜单：`role_menus`
3. 菜单-API：`menu_apis`
4. 角色-API（直绑）：`role_apis`
5. 角色-capability：`role_capabilities`

### 2.2 权限真相与投影

1. DB 关系表是唯一业务真相。
2. Casbin 只保存权限投影，用于 API / capability 访问判断。
3. `AuthorizationService` 负责 owner / super_admin / capability / API 鉴权。
4. `PermissionProjectionService` 负责 Casbin 重建、主体修复、事件消费和 reload 广播。

### 2.3 最终权限来源（并集）

系统会同步两路 API 权限到 Casbin：

1. 菜单链路：`role -> menu -> api`
2. 角色直绑：`role -> api`

因此最终意图是：

- 用户在某组织下的可访问 API = 菜单链路 API 与角色直绑 API 的并集。

---

## 3. 数据模型

### 3.1 关键业务表

1. `users`
   - `current_org_id`：当前组织上下文（用于生成 `subject`）
2. `roles`
   - `code`：角色唯一编码（如 `super_admin` / `org_admin` / `member`）
3. `menus`
   - `code`：菜单编码
4. `apis`
   - `path + method` 联合唯一
5. `user_org_roles`
   - 用户在组织下拥有的角色关系（多租户核心）
6. `role_menus`
   - 角色菜单授权关系（GORM many2many 自动中间表）
7. `menu_apis`
   - 菜单 API 绑定关系（手动 SQL 维护）
8. `role_apis`
   - 角色 API 直绑关系（`entity.RoleAPI`）

### 3.2 Casbin 持久化表

Casbin 使用 `gorm-adapter`，策略持久化在 `casbin_rule`。

---

## 4. Casbin 初始化与权限投影

### 4.1 初始化

启动时在 `internal/core/casbin.go`：

1. 读取 `configs/model.conf`
2. 使用 `gorm-adapter` 绑定数据库
3. 创建 `Enforcer`
4. 开启 `EnableAutoSave(true)`

### 4.2 启动基线重建

在 `internal/init/init.go` 启动阶段调用：

- `permissionProjectionService.RebuildAll(ctx)`

冷启动基线：

1. 从 DB 回源读取 `user_org_roles / role_menus / menu_apis / role_apis / role_capabilities`
2. 组装权限快照
3. 执行 `pkg/casbin.RebuildFromSnapshot`

### 4.3 运行时事件链路

1. 主体绑定类变更：
   - 事务内写 DB + `subject_binding_changed` outbox
   - 事务提交后同步当前实例 `SyncSubjectRoles`
   - 异步消费时再按 `user_id + org_id` 回源修复
2. 权限图谱类变更：
   - 事务内写 DB + `permission_graph_changed` outbox
   - 异步消费时执行 Casbin 全量重建
3. 消费成功后广播 `permission_policy_reload`
4. 其他实例收到广播后执行 `ReloadPolicy`

---

## 5. 运行时鉴权链路

### 5.1 路由分组

`internal/router/router.go` 中：

1. `SystemGroup`：`JWTAuth + PermissionMiddleware`
   - 包含 `system/api`、`system/menu`、`system/role`、`system/org`（管理类）
2. `BusinessGroup`：仅 `JWTAuth`
   - 包含 OJ、图片、组织切换等业务路由

### 5.2 权限中间件流程

`internal/middleware/permissionMW.go`：

1. 白名单放行
2. 读取 JWT claims，拿 `userID`
3. 加载用户角色（`AuthorizationService.GetUserRoles`）
4. 若包含 `super_admin`，直接放行
5. 非超管执行 API 权限校验：
   - `apiPath = c.FullPath()`（优先路由模板）
   - 调 `AuthorizationService.CheckUserAPIPermission(ctx, userID, apiPath, method)`

### 5.3 subject 生成

`AuthorizationService.getUserSubject`：

1. 读取 `users.current_org_id`
2. 通过 `pkg/casbin.BuildSubject` 生成 `subject = userID@orgID`
3. 若未设置当前组织，则返回错误

---

## 6. 多租户行为说明

### 6.1 组织维度隔离

同一用户在不同组织的权限可不同：

1. 在 org A：`subject = 39@2`
2. 在 org B：`subject = 39@3`

即使用户 ID 相同，Casbin 主体不同，权限判断结果可以不同。

### 6.2 全局角色

`org_id = 0` 视为全局角色（如 `super_admin`）。

`GetUserRoles` 逻辑：

1. 先查全局角色
2. 若存在 `super_admin`，直接返回（并在中间件直接放行）
3. 否则再按 `current_org_id` 查组织内角色并合并

### 6.3 Service / Action 层资源授权

对组织域写操作，除 API 中间件外，Service 层还会做一次资源级授权：

1. 入口统一收口到 `AuthorizationService.AuthorizeOrgCapability`
2. 若操作者是目标组织 owner，直接放行
3. 若操作者拥有全局 `super_admin`，直接放行
4. 其余按 `subject = userID@orgID` 检查 capability
5. 无 capability 返回 `CodePermissionDenied`

当前已接入的组织域 capability 分组：

1. `org_member_management`
   - `org.member.kick`
   - `org.member.recover`
   - `org.member.freeze`
   - `org.member.delete`
   - `org.member.invite`
   - `org.member.assign_role`
2. `org_management`
   - `org.manage.update`
   - `org.manage.delete`

---

## 7. 权限写入入口与一致性策略

### 7.1 主体绑定类变更

入口示例：注册、入组、退组、踢人、恢复成员、改角色

1. 事务内先写 DB
2. 同事务写 `subject_binding_changed` outbox
3. 提交后同步当前实例 `SyncSubjectRoles`
4. 异步消费者兜底修复全部实例

### 7.2 权限图谱类变更

入口示例：角色分配菜单/API/capability、菜单绑定 API、API 变更

1. 事务内更新关系表
2. 同事务写 `permission_graph_changed` outbox
3. 异步消费者执行 Casbin 全量重建
4. 重建完成后广播 `permission_policy_reload`

### 7.3 失败语义

1. 事务提交成功后，即使投影消费延迟，DB 仍是唯一真相。
2. 当前实例主体绑定写路径会同步收口，保证本实例即时生效。
3. 图谱类变更通过异步重建和 reload 广播在多实例间收敛；错过广播的实例由下次变更或重启时基线重建修复。

---

## 8. 角色映射大对象接口

### 8.1 路径

- `GET /system/role/{id}/menu_api_map`

### 8.2 当前响应语义

该接口当前返回：

- `menu_tree`（全量菜单树，节点内含 `apis`）
- `assigned_menu_ids`
- `direct_api_ids`
- `assigned_api_ids`
- `capability_groups`
- `assigned_capability_codes`

说明：

1. 已用于权限页“一次性渲染树结构和当前选中态”。
2. 前端无需再额外调用独立的菜单/API/ capability 查询接口拼装配置态数据。

### 8.3 用户角色矩阵接口

- `GET /system/user/{id}/role_matrix?org_id={orgID}`

该接口用于“给用户分配角色”弹窗的一次性渲染，当前返回：

- `assigned_role_ids`
- `operator_matrix_level`
- `roles[]`

其中 `roles[]` 的每一项固定包含：

- `id`
- `name`
- `code`
- `is_builtin`
- `matrix_level`
- `assignable`
- `disabled_reason`

当前实现语义：

1. 这是一份组织内角色分配的只读矩阵，不参与角色权限配置页。
2. 读写两条链路都先走 `AuthorizationService.AuthorizeOrgCapability(..., org.member.assign_role)`。
3. 操作者层级只认三档：`super_admin / org_admin / member`。
4. 自定义角色全部按 `member` 层处理，彼此平级。
5. `super_admin` 仍是全局角色，只允许绑定到 `org_id=0`；因此在该接口中会返回，但固定 `assignable=false`、`disabled_reason=global_role_only`。
6. `AssignRole` 会复用同一套矩阵规则校验 `role_ids`，避免前端可见与后端可写不一致。

---

## 9. API 生命周期与权限关系清理

### 9.1 删除单个 API

`ApiService.DeleteAPI` 顺序：

1. `menuRepo.RemoveAPIFromAllMenus(apiID)`
2. `roleRepo.RemoveAPIFromAllRoles(apiID)`
3. `apiRepo.Delete(apiID)`

### 9.2 路由同步删除（`SyncAPI(deleteRemoved=true)`）

对“路由已不存在”的 API 采用同样顺序先解绑关系，再删 API，避免关系脏数据和外键问题。

---

## 10. 迁移与内置角色

### 10.1 自动迁移

`flag/flagSql.go` 会迁移：

1. `RoleAPI`（`role_apis`）
2. 其他权限相关实体（`Role`、`Menu`、`API`、`UserOrgRole` 等）

### 10.2 内置角色初始化（幂等）

迁移后会自动确保以下角色存在：

1. `super_admin`
2. `org_admin`
3. `member`

---

## 11. 关键配置项

1. `configs/model.conf`：Casbin 模型定义
2. `system.auto_migrate`：是否自动迁移表结构
3. `system.default_role_code`：注册时默认角色编码

---

## 12. 部署校验清单（强烈建议）

每次发布前至少检查：

1. `user_org_roles`、`role_menus`、`menu_apis`、`role_apis` 数据是否完整。
2. `role_capabilities` 数据是否完整。
2. `users.current_org_id` 是否正确维护（否则 subject 无法生成）。
3. 启动日志中 `RebuildAll` 是否成功。
4. `subject_binding_changed / permission_graph_changed` 是否成功写出并被消费。
5. `permission_policy_reload` 广播后其他实例是否成功 `ReloadPolicy`。

### 12.1 模型维度一致性校验（重要）

当前代码在多处使用三元调用：

- `Enforce(subject, resource, "access")`
- `AddPermissionForUser(..., ..., "read"/"access")`

请确保 `configs/model.conf` 的 `r`/`p` 维度与调用维度一致；否则会出现策略尺寸不匹配，导致权限校验异常。

---

## 13. 调试建议

1. 查用户主体：确认 `user_id` 对应的 `current_org_id`。
2. 查角色关系：`user_org_roles` 是否有 `user_id + org_id` 记录。
3. 查菜单链路：`role_menus`、`menu_apis` 是否贯通。
4. 查直绑链路：`role_apis` 是否有目标 API。
5. 查 capability 链路：`role_capabilities` 是否有目标 capability。
6. 必要时执行一次基线重建：`PermissionProjectionService.RebuildAll`。

---

**版本**: v1.0（实现对齐版）  
**最后更新**: 2026-02-25  
**维护者**: 王得贤 / 项目协作机器人
