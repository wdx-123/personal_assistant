# 实现计划：用户管理模块（Casbin RBAC）

## 概述

按照项目规范的实现顺序（DTO → Repository → Service → Controller → Router），逐步实现用户列表、用户详情、用户组织角色查询、角色分配四个 API 接口。每一步构建在前一步之上，最终通过路由注册将所有功能串联。

## 任务

- [x] 1. 定义请求和响应 DTO
  - [x] 1.1 在 `internal/model/dto/request/userReq.go` 中新增 `UserListReq` 和 `AssignUserRoleReq` 结构体
    - `UserListReq`：Page、PageSize（带默认值）、OrgID、Keyword，使用 `form` tag 绑定 query 参数
    - `AssignUserRoleReq`：UserID、OrgID、RoleIDs，使用 `json` tag 和 `binding:"required"` 校验
    - _Requirements: 1.1, 1.2, 1.3, 4.1_
  - [x] 1.2 在 `internal/model/dto/response/userResp.go` 中新增 `UserListItem` 结构体
    - 包含 id、username、phone、current_org（嵌套 id+name）、roles（嵌套 id+name 数组）
    - _Requirements: 1.5_

- [-] 2. 扩展 Repository 层
  - [ ] 2.1 在 `internal/repository/interfaces/userRepository.go` 中新增 `GetUserListWithFilter` 方法签名
    - 签名：`GetUserListWithFilter(ctx context.Context, filter *request.UserListReq) ([]*entity.User, int64, error)`
    - _Requirements: 1.2, 1.3, 1.6_
  - [~] 2.2 在 `internal/repository/system/userRepo.go` 中实现 `GetUserListWithFilter`
    - 支持 org_id 过滤（通过 current_org_id）、keyword 模糊搜索（username/phone）
    - Preload CurrentOrg 关联
    - 处理默认分页值（page=1, page_size=10）
    - _Requirements: 1.1, 1.2, 1.3, 1.6, 1.7_
  - [~] 2.3 在 `internal/repository/interfaces/roleRepository.go` 中新增 `GetByIDs` 方法签名
    - 签名：`GetByIDs(ctx context.Context, ids []uint) ([]*entity.Role, error)`
    - _Requirements: 4.5_
  - [~] 2.4 在 `internal/repository/system/roleRepo.go` 中实现 `GetByIDs`
    - 批量查询角色，用于角色分配时校验角色是否存在
    - _Requirements: 4.5_

- [ ] 3. 扩展 Service 层
  - [~] 3.1 在 `internal/service/system/userSvc.go` 中为 `UserService` 新增 `orgRepo` 依赖
    - 更新 `UserService` 结构体，添加 `orgRepo interfaces.OrgRepository` 字段
    - 更新 `NewUserService` 构造函数，从 repositoryGroup 获取 orgRepo
    - 更新 `internal/service/system/supplier.go` 中 `SetUp` 函数的 `NewUserService` 调用
    - _Requirements: 3.4, 4.4_
  - [~] 3.2 实现 `UserService.GetUserList` 方法
    - 调用 `userRepo.GetUserListWithFilter` 获取用户列表
    - 批量查询用户在当前组织下的角色（通过 `roleRepo.GetUserRolesByOrg`）
    - 将 Entity 转换为 `UserListItem` DTO
    - _Requirements: 1.1, 1.2, 1.3, 1.5, 1.6, 1.7_
  - [~] 3.3 实现 `UserService.GetUserDetail` 方法
    - 调用 `userRepo.GetByID` 获取用户
    - 用户不存在时返回 `errors.New(errors.CodeUserNotFound)`
    - 将 Entity 转换为 `UserDetailItem` DTO
    - _Requirements: 2.1, 2.2, 2.3_
  - [~] 3.4 实现 `UserService.GetUserRolesByOrg` 方法
    - 校验用户存在性（userRepo.GetByID）
    - 校验组织存在性（orgRepo.GetByID）
    - 调用 `roleRepo.GetUserRolesByOrg` 获取角色列表
    - 将 Entity 转换为 `RoleSimpleItem` DTO
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_
  - [~] 3.5 实现 `UserService.AssignUserRole` 方法
    - 校验用户、组织、角色存在性
    - 获取当前角色列表，计算差异（使用 `pkg/util/diffArrays.go`）
    - 逐条调用 `permissionService.RemoveRoleFromUserInOrg` 移除旧角色
    - 逐条调用 `permissionService.AssignRoleToUserInOrg` 添加新角色
    - 任一步骤失败时回滚已完成的操作
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_

- [~] 4. Checkpoint - 确保 Service 层逻辑完整
  - 确保所有测试通过，如有疑问请向用户确认。

- [ ] 5. 扩展 Controller 层
  - [~] 5.1 在 `internal/controller/system/userCtrl.go` 中为 `UserCtrl` 新增 `permissionService` 依赖
    - 更新结构体和 `internal/controller/system/supplier.go` 中的初始化代码
    - _Requirements: 6.1_
  - [~] 5.2 实现 `UserCtrl.GetUserList` handler
    - 使用 `c.ShouldBindQuery` 绑定 `UserListReq`
    - 调用 `userService.GetUserList`
    - 使用 `response.BizOkWithPage` 返回分页数据
    - _Requirements: 1.1, 1.6, 5.1, 5.2, 5.4_
  - [~] 5.3 实现 `UserCtrl.GetUserDetail` handler
    - 从 URL 路径解析 id 参数
    - 调用 `userService.GetUserDetail`
    - 使用 `response.BizOkWithData` 返回详情
    - _Requirements: 2.1, 2.3, 5.1, 5.3_
  - [~] 5.4 实现 `UserCtrl.GetUserRoles` handler
    - 从 URL 路径解析 id，从 query 解析 org_id
    - 调用 `userService.GetUserRolesByOrg`
    - 使用 `response.BizOkWithData` 返回角色列表
    - _Requirements: 3.1, 3.3, 3.4, 5.1, 5.3_
  - [~] 5.5 实现 `UserCtrl.AssignUserRole` handler
    - 使用 `c.ShouldBindJSON` 绑定 `AssignUserRoleReq`
    - 调用 `userService.AssignUserRole`
    - 使用 `response.BizOkWithMessage` 返回成功消息
    - _Requirements: 4.1, 4.3, 5.1, 5.2, 5.5_

- [ ] 6. 注册路由
  - [~] 6.1 在 `internal/router/system/userRouter.go` 中新增四个路由
    - GET `list` → `userCtrl.GetUserList`
    - GET `:id` → `userCtrl.GetUserDetail`
    - GET `:id/roles` → `userCtrl.GetUserRoles`
    - POST `assign_role` → `userCtrl.AssignUserRole`
    - 将新路由注册到需要 JWT 认证和权限中间件的路由组中
    - _Requirements: 1.1, 2.1, 3.1, 4.1_
  - [~] 6.2 在 `internal/router/router.go` 中将用户管理路由注册到 SystemGroup（需要 JWT + 权限中间件）
    - 新增 `InitUserManageRouter` 调用，挂载到 SystemGroup
    - 保持原有 `InitUserRouter`（登录/注册等公开接口）不变
    - _Requirements: 6.1_

- [~] 7. Checkpoint - 确保所有代码编译通过且路由正确注册
  - 确保所有测试通过，如有疑问请向用户确认。

- [ ]* 8. 编写属性测试
  - [ ]* 8.1 编写 Property 1 属性测试：组织过滤正确性
    - **Property 1: 组织过滤正确性**
    - 生成随机用户数据集和 org_id，验证过滤结果中所有用户的 current_org_id 等于 org_id
    - **Validates: Requirements 1.2**
  - [ ]* 8.2 编写 Property 2 属性测试：关键词过滤正确性
    - **Property 2: 关键词过滤正确性**
    - 生成随机用户数据集和 keyword，验证过滤结果中所有用户的 username 或 phone 包含 keyword
    - **Validates: Requirements 1.3**
  - [ ]* 8.3 编写 Property 3 属性测试：用户列表项字段完整性
    - **Property 3: 用户列表项字段完整性**
    - 生成随机用户 Entity，验证 DTO 转换后所有字段值与源数据一致
    - **Validates: Requirements 1.5**
  - [ ]* 8.4 编写 Property 4 属性测试：用户详情查询一致性
    - **Property 4: 用户详情查询一致性**
    - 生成随机用户 Entity，验证详情 DTO 转换后所有字段值与源数据一致
    - **Validates: Requirements 2.1, 2.2**
  - [ ]* 8.5 编写 Property 5 属性测试：用户组织角色查询一致性
    - **Property 5: 用户组织角色查询一致性**
    - 生成随机用户-组织-角色关联，验证查询结果与数据库数据一致
    - **Validates: Requirements 3.1, 3.2**
  - [ ]* 8.6 编写 Property 6 属性测试：角色分配 round-trip
    - **Property 6: 角色分配 round-trip**
    - 生成随机用户、组织和角色列表，执行分配后查询，验证结果与传入的 role_ids 一致
    - **Validates: Requirements 4.1, 4.2**

- [ ]* 9. 编写单元测试
  - [ ]* 9.1 编写 Service 层单元测试
    - 测试用户不存在返回 20001 错误
    - 测试组织不存在返回 30001 错误
    - 测试角色不存在返回 30101 错误
    - 测试空结果返回空列表
    - 测试空 role_ids 清除所有角色
    - 测试 Casbin 失败回滚
    - _Requirements: 2.3, 3.3, 3.4, 4.3, 4.4, 4.5, 4.6, 4.7_
  - [ ]* 9.2 编写 Controller 层单元测试
    - 测试参数绑定失败返回 10002
    - 测试必需参数缺失返回 10003
    - 测试默认分页参数
    - _Requirements: 1.1, 5.4, 5.5_

- [~] 10. Final checkpoint - 确保所有测试通过
  - 确保所有测试通过，如有疑问请向用户确认。

## 备注

- 标记 `*` 的任务为可选任务，可跳过以加速 MVP 交付
- 每个任务引用了具体的需求编号，确保可追溯性
- Checkpoint 任务确保增量验证
- 属性测试验证通用正确性属性，单元测试验证具体示例和边界情况
