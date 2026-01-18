# 项目转型指南：个人博客 -> 刷题检查平台

## 1. 项目结构调整分析 (Cleanup Analysis)

本指南旨在帮助将现有的 `personal_blog` 项目改造为刷题检查平台。

### 1.1 需要保留的模块 (Keep)
这些模块是项目的基础设施，应当保留并复用：
*   **`pkg/`**: 工具包 (JWT, Logger, Response, Database Drivers, Utils)。
*   **`configs/`**: 配置文件结构 (YAML, Viper setup)。
*   **`internal/core/`**: 核心启动逻辑 (Server, Zap, Gorm, Redis 初始化)。
*   **`internal/middleware/`**: 中间件 (Auth, CORS, Logger, Timeout, Casbin/RBAC)。
*   **`internal/router/`**: 路由基础结构 (虽然具体的路由需要修改)。
*   **`internal/model/entity/`**:
    *   `base.go` (基础 Model 结构)
    *   `role.go` (角色管理)
    *   `menu.go` (如果需要后台菜单权限)
    *   `jwt.go` / `login.go` (认证相关)
    *   `user.go` (**重点修改**: 需要保留基础字段，但去除博客相关，增加组织和刷题平台关联)

### 1.2 需要删除/重构的模块 (Delete/Refactor)
这些模块与新业务无关，建议删除或清空逻辑：
*   **实体 (Entities)**:
    *   `internal/model/entity/article*.go` (文章, 分类, 标签, 点赞)
    *   `internal/model/entity/image.go` (如果新项目不需要复杂的图片库，可简化或删除)
*   **控制器 (Controllers)**:
    *   `internal/controller/system/articleCtrl.go`
    *   `internal/controller/system/imageCtrl.go` (视需求而定)
*   **服务 (Services)**:
    *   `internal/service/system/articleSvc.go`
    *   `internal/service/system/imageSvc.go`
*   **仓库 (Repositories)**:
    *   对应上述实体的数据层代码。

---

## 2. 数据库设计与字段定义 (Database Schema & Fields)

基于 Go + Gorm + Gin 开发，以下是核心业务表的设计建议。

### 2.1 核心基础表

#### A. 组织/班级表 (Organization)
用于支持 "分组织" 功能。
```go
type Organization struct {
    global.MODEL
    Name        string `json:"name" gorm:"type:varchar(100);not null;comment:组织名称"`
    Description string `json:"description" gorm:"type:varchar(255);comment:组织描述"`
    Code        string `json:"code" gorm:"type:varchar(20);index;comment:加入邀请码"`
    OwnerID     uint   `json:"owner_id" gorm:"index;comment:创建者/负责人ID"`
}
```

#### B. 用户表 (User) - 改造
基于现有 `User` 表进行扩展。
```go
type User struct {
    global.MODEL
    UUID           uuid.UUID `json:"uuid" gorm:"type:char(36);unique;not null"`
    Username       string    `json:"username" gorm:"type:varchar(50);unique;not null"`
    Password       string    `json:"-" gorm:"type:varchar(255);not null"`
    Nickname       string    `json:"nickname" gorm:"type:varchar(50);comment:昵称"`
    Avatar         string    `json:"avatar" gorm:"type:varchar(255);comment:头像"`
    Email          string    `json:"email" gorm:"type:varchar(100);index"`
    RoleID         uint      `json:"role_id" gorm:"default:1;comment:角色ID"` // 关联 Role 表
    OrganizationID uint      `json:"organization_id" gorm:"index;comment:所属组织ID"`
    
    // 关联信息 (Preload 使用)
    Organization Organization `json:"organization" gorm:"foreignKey:OrganizationID"`
    Role         Role         `json:"role" gorm:"foreignKey:RoleID"`
    
    // 平台绑定信息 (一对一)
    LeetCodeBind *LeetCodeBind `json:"leetcode_bind" gorm:"foreignKey:UserID"`
    LuoguBind    *LuoguBind    `json:"luogu_bind" gorm:"foreignKey:UserID"`
}
```

### 2.2 刷题平台绑定表

#### C. 力扣绑定信息 (LeetCodeBind)
```go
type LeetCodeBind struct {
    global.MODEL
    UserID        uint   `json:"user_id" gorm:"uniqueIndex;not null"`
    LeetCodeName  string `json:"leetcode_name" gorm:"type:varchar(100);comment:力扣用户名/Slug"`
    Avatar        string `json:"avatar" gorm:"type:varchar(255)"`
    Ranking       int    `json:"ranking" gorm:"comment:全站排名"`
    Rating        int    `json:"rating" gorm:"comment:竞赛积分"`
    SolvedEasy    int    `json:"solved_easy" gorm:"comment:简单题解题数"`
    SolvedMedium  int    `json:"solved_medium" gorm:"comment:中等题解题数"`
    SolvedHard    int    `json:"solved_hard" gorm:"comment:困难题解题数"`
    LastSyncTime  time.Time `json:"last_sync_time" gorm:"comment:上次同步时间"`
}
```

#### D. 洛谷绑定信息 (LuoguBind)
```go
type LuoguBind struct {
    global.MODEL
    UserID        uint   `json:"user_id" gorm:"uniqueIndex;not null"`
    LuoguID       string `json:"luogu_id" gorm:"type:varchar(50);comment:洛谷UID"`
    LuoguName     string `json:"luogu_name" gorm:"type:varchar(100);comment:洛谷用户名"`
    Avatar        string `json:"avatar" gorm:"type:varchar(255)"`
    PassedProblemCount int `json:"passed_problem_count" gorm:"comment:通过题目数"`
    Ranking       int    `json:"ranking" gorm:"comment:排名"`
    LastSyncTime  time.Time `json:"last_sync_time" gorm:"comment:上次同步时间"`
}
```

### 2.3 题目与记录 (核心业务)

#### E. 综合题库表 (ProblemRepository)
存放力扣和洛谷的所有题目元数据。
```go
type ProblemRepository struct {
    global.MODEL
    Platform    int    `json:"platform" gorm:"type:tinyint;index;comment:平台(1:LeetCode, 2:Luogu)"`
    ProblemIdentify string `json:"problem_identify" gorm:"type:varchar(100);index;comment:题目原始ID(如 two-sum 或 P1001)"`
    Title       string `json:"title" gorm:"type:varchar(255);index;comment:题目标题"`
    Difficulty  string `json:"difficulty" gorm:"type:varchar(20);comment:难度(Easy/Medium/Hard 或 洛谷难度)"`
    URL         string `json:"url" gorm:"type:varchar(500);comment:题目链接"`
    Tags        string `json:"tags" gorm:"type:json;comment:标签(JSON存储)"` 
}
```

#### F. 用户刷题记录 (UserProblemRecord)
记录谁做了哪道题。
```go
type UserProblemRecord struct {
    global.MODEL
    UserID      uint   `json:"user_id" gorm:"index;uniqueIndex:idx_user_problem"`
    ProblemID   uint   `json:"problem_id" gorm:"index;uniqueIndex:idx_user_problem;comment:关联ProblemRepository"`
    Platform    int    `json:"platform" gorm:"type:tinyint;comment:冗余字段方便查询"`
    SolvedAt    time.Time `json:"solved_at" gorm:"index;comment:通过时间"`
    Status      int    `json:"status" gorm:"default:1;comment:状态(1:已通过, 2:尝试中)"`
}
```

### 2.4 检查/任务平台 (Task/Check)

#### G. 刷题任务/作业 (Task)
```go
type Task struct {
    global.MODEL
    OrganizationID uint   `json:"organization_id" gorm:"index"`
    Title          string `json:"title" gorm:"type:varchar(100);not null"`
    Description    string `json:"description" gorm:"type:text"`
    StartDate      time.Time `json:"start_date"`
    EndDate        time.Time `json:"end_date"`
    TargetProblems string `json:"target_problems" gorm:"type:json;comment:目标题目ID列表(JSON数组)"`
}
```

#### H. 任务完成情况 (UserTaskStatus)
```go
type UserTaskStatus struct {
    global.MODEL
    TaskID      uint `json:"task_id" gorm:"index;uniqueIndex:idx_user_task"`
    UserID      uint `json:"user_id" gorm:"index;uniqueIndex:idx_user_task"`
    IsCompleted bool `json:"is_completed" gorm:"default:false"`
    Progress    string `json:"progress" gorm:"type:varchar(50);comment:进度描述(如 5/10)"`
}
```

---

## 3. 开发建议

1.  **数据获取**: 需要编写爬虫或对接 API (LeetCode GraphQL API, 洛谷页面解析) 来定期同步用户的做题数据。建议使用 `colly` 或标准 `net/http` 库。
2.  **定时任务**: 利用现有的 `corn` (cron) 模块，设置每日/每小时任务同步 UserProblemRecord。
3.  **权限控制**: 利用 `Casbin` 或简单的中间件，确保只有 Organization 的管理员(Admin/Teacher)可以发布 Task。
