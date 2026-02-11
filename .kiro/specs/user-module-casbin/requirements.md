# 需求文档

## 简介

基于 Casbin RBAC 权限系统，实现用户管理模块的四个核心 API 接口：用户列表查询（分页、过滤）、用户详情查询、用户组织角色查询、用户角色分配（全量替换）。系统遵循分层架构（Controller → Service → Repository），使用 Gin + GORM + Casbin 技术栈，通过 `UserOrgRole` 三元组实现多租户角色管理。

## 术语表

- **User_Module**：用户管理模块，负责用户信息查询与角色分配的后端服务
- **Controller**：控制器层，仅负责请求接收、参数校验、调用 Service、组装响应
- **Service**：服务层，负责业务编排，通过调用 Repository 完成数据操作
- **Repository**：仓储层，负责全部数据库交互（CRUD / JOIN）
- **UserOrgRole**：用户-组织-角色三元关联表，实现"用户在某组织下拥有某角色"的多租户模型
- **Casbin**：基于策略的访问控制框架，管理用户角色与权限策略
- **BizError**：业务错误类型，由 Service 层创建，包含错误码和用户可读消息
- **DTO**：数据传输对象，用于 API 请求和响应的数据结构定义

## 需求

### 需求 1：获取用户列表

**用户故事：** 作为系统管理员，我希望分页获取用户列表并支持按组织和关键词过滤，以便管理系统中的用户。

#### 验收标准

1. WHEN 客户端发送 GET `/system/user/list` 请求且未提供分页参数时，THE User_Module SHALL 使用默认值 page=1、page_size=10 返回用户列表
2. WHEN 客户端提供 org_id 参数时，THE User_Module SHALL 仅返回属于该组织的用户
3. WHEN 客户端提供 keyword 参数时，THE User_Module SHALL 返回用户名或手机号包含该关键词的用户
4. WHEN 客户端同时提供 org_id 和 keyword 参数时，THE User_Module SHALL 返回同时满足组织过滤和关键词匹配的用户
5. THE User_Module SHALL 在用户列表响应中包含每个用户的 id、username、phone、current_org（id 和 name）以及 roles（id 和 name）字段
6. THE User_Module SHALL 在用户列表响应中包含分页信息：list、total、page、page_size
7. WHEN 查询结果为空时，THE User_Module SHALL 返回空列表和 total=0

### 需求 2：获取用户详情

**用户故事：** 作为系统管理员，我希望根据用户 ID 获取用户的详细信息，以便查看和管理用户资料。

#### 验收标准

1. WHEN 客户端发送 GET `/system/user/{id}` 请求且用户存在时，THE User_Module SHALL 返回该用户的完整详情信息
2. THE User_Module SHALL 在用户详情响应中包含 id、uuid、username、phone、email、avatar、address、signature、register、freeze、current_org（id 和 name）、created_at、updated_at 字段
3. IF 请求的用户 ID 对应的用户不存在，THEN THE User_Module SHALL 返回错误码 20001 和消息"用户不存在"

### 需求 3：获取用户在组织下的角色

**用户故事：** 作为系统管理员，我希望查询用户在指定组织下的角色列表，以便了解用户的权限配置。

#### 验收标准

1. WHEN 客户端发送 GET `/system/user/{id}/roles` 请求并提供必需的 org_id 参数时，THE User_Module SHALL 返回该用户在指定组织下的角色列表
2. THE User_Module SHALL 在角色列表响应中包含每个角色的 id、name、code 字段
3. IF 请求的用户 ID 对应的用户不存在，THEN THE User_Module SHALL 返回错误码 20001 和消息"用户不存在"
4. IF 请求的 org_id 对应的组织不存在，THEN THE User_Module SHALL 返回错误码 30001 和消息"组织不存在"
5. WHEN 用户在指定组织下没有任何角色时，THE User_Module SHALL 返回空角色列表

### 需求 4：分配用户角色

**用户故事：** 作为系统管理员，我希望在指定组织下为用户分配角色（全量替换），以便灵活管理用户权限。

#### 验收标准

1. WHEN 客户端发送 POST `/system/user/assign_role` 请求并提供有效的 user_id、org_id、role_ids 时，THE User_Module SHALL 全量替换该用户在该组织下的角色
2. WHEN 角色分配成功时，THE User_Module SHALL 同步更新 Casbin 策略，确保权限立即生效
3. IF 请求的 user_id 对应的用户不存在，THEN THE User_Module SHALL 返回错误码 20001 和消息"用户不存在"
4. IF 请求的 org_id 对应的组织不存在，THEN THE User_Module SHALL 返回错误码 30001 和消息"组织不存在"
5. IF 请求的 role_ids 中包含不存在的角色 ID，THEN THE User_Module SHALL 返回错误码 30101 和消息"角色不存在"
6. WHEN role_ids 为空数组时，THE User_Module SHALL 清除该用户在该组织下的所有角色并同步更新 Casbin 策略
7. IF 数据库操作成功但 Casbin 策略同步失败，THEN THE User_Module SHALL 回滚数据库操作并返回错误

### 需求 5：统一响应格式与错误处理

**用户故事：** 作为 API 消费者，我希望所有接口返回统一的响应格式，以便前端统一处理。

#### 验收标准

1. THE User_Module SHALL 使用统一响应结构：code、success、message、data、timestamp
2. WHEN 操作成功时，THE User_Module SHALL 返回 code=0 和 success=true
3. WHEN 操作失败时，THE User_Module SHALL 返回对应的业务错误码和 success=false
4. THE User_Module SHALL 对所有请求参数执行校验，参数绑定失败时返回错误码 10002
5. IF 请求缺少必需参数（如 assign_role 的 user_id、org_id、role_ids），THEN THE User_Module SHALL 返回错误码 10003 和参数校验失败消息

### 需求 6：分层架构合规

**用户故事：** 作为开发者，我希望用户模块严格遵循分层架构规范，以便代码可维护和可测试。

#### 验收标准

1. THE Controller SHALL 仅执行请求接收、参数校验、调用 Service 和组装响应，禁止包含业务逻辑或直接访问数据库
2. THE Service SHALL 仅执行业务编排，禁止直接访问数据库，所有数据操作通过 Repository 完成
3. THE Repository SHALL 负责全部数据库交互，仅返回原始 error，不感知业务场景
4. THE User_Module SHALL 使用 DTO 作为 API 请求和响应的数据结构，禁止直接使用 Entity
