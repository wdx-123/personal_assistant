# README 重写方案（展示 + 面试取向）

## Summary

- 将 `README.md` 从“基础启动说明”升级为“项目入口页”，主线固定为 **Go 后端项目介绍**，优先服务 GitHub 展示和面试讲解。
- 改造方式采用 **整体重写**：保留现有可用的快速开始、Docker、CLI、相关文档、安全提醒，但按当前真实代码状态重排结构。
- 所有内容以代码为准，不补脑、不拔高；明确区分：
  - **代码已落地**
  - **待确认 / 规划中**
  - **外部依赖边界**

## Key Changes

- README 统一使用中文，语气改为“架构先行、模块清晰、可直接拿来讲项目”，不写成营销文案。
- 开头改成 3 段式：
  - 一句话介绍项目是什么
  - 这个项目解决什么问题
  - 为什么它不是简单 CRUD
- 章节结构固定为下面这版，不再沿用当前“亮点堆列表”的写法：

1. 项目简介  
   写一句话介绍 + 2~3 句定位说明，明确这是 **模块化单体 Go 后端**，不是微服务，也不是纯后台管理系统。

2. 项目定位  
   说明它覆盖：
   - 用户/组织/RBAC 权限治理
   - OJ 数据同步与任务化运营
   - AI 会话/SSE 助手能力
   - 图片与可观测性等工程治理  
   这里要直接说“更接近业务型平台后端”。

3. 核心能力  
   分三组写，不再混排：
   - 核心功能：用户认证、组织管理、RBAC、OJ、OJ 任务、AI 会话
   - 支撑能力：图片、SSE、可观测性、限流、缓存
   - 工程能力：Outbox、Redis Stream、分布式锁、自动迁移、CLI、Docker  
   每项只写 1 句作用，不展开成审计报告。

4. 架构总览  
   明确写出架构结论：
   - 模块化单体
   - 分层架构 / Handler-Service-Repository
   - 业务真相在 MySQL，投影在 Redis/Casbin
   - 异步解耦采用 Outbox + Redis Stream + Pub/Sub
   - AI 子域采用 Eino runtime + SSE 单流  
   用一段文字图表示启动链：  
   `cmd/main -> internal/init -> core/infrastructure -> repository -> service -> controller -> router -> gin server`

5. 核心模块  
   按模块写职责，不写目录树大抄：
   - 启动与基础设施
   - 接口层
   - 用户与认证
   - 组织与权限
   - OJ 业务
   - OJ 任务
   - AI/Agent
   - 图片与存储
   - 可观测性
   - 消息与异步  
   每个模块写“职责 + 关键目录”两部分即可。

6. 核心链路  
   固定写三条：
   - 请求流
   - 数据流
   - 异步流  
   再补 2~4 个典型业务链路：
   - 注册
   - OJ 绑定
   - OJ 任务执行
   - AI 会话流式输出

7. 技术栈与依赖  
   保留当前技术栈，但改成“框架 / 存储 / 权限 / 异步 / AI / 工程化”分组。  
   依赖边界要明确写：
   - MySQL、Redis 为必需
   - OJ crawler 为可选外部依赖
   - 七牛为可选存储驱动

8. 快速开始  
   保留并校正现有内容：
   - 前置依赖
   - `.env.example -> .env`
   - 本地启动
   - 自动迁移 / `--sql`
   - Docker 启动  
   这里不扩写部署理论，只保留可执行说明。

9. 配置说明  
   不枚举所有 env，但要归纳配置组：
   - System / JWT / Session
   - MySQL / Redis
   - Storage / Static
   - Crawler
   - Observability
   - Task / Messaging / SSE / AI  
   并说明“具体键以 `.env.example` 和 `configs/configs.yaml` 为准”。

10. 接口分组概览  
   用“分组 + 示例接口”的方式重写，不再只列老接口。至少覆盖：
   - 健康检查
   - 登录注册与刷新
   - 用户业务
   - 组织业务
   - 系统权限管理
   - OJ
   - OJ Task
   - AI 会话 / SSE
   - 图片
   - 可观测性  
   继续保留“当前前缀不完全统一”的提醒，不伪装成已完全规范化。

11. 当前完成度与边界  
   新增一节，明确：
   - 已落地主链路：用户/组织/权限、OJ、OJ 任务、AI 会话、图片、观测
   - 不宜过度表述：完整多 Agent 平台、微服务化、完整 OpenAPI 治理
   - 外部依赖边界：OJ 数据依赖 crawler

12. 相关文档  
   保留并扩充 docs 索引，优先挂这些：
   - 事件驱动架构
   - Casbin RBAC
   - 双 Token
   - AI 架构设计
   - SSE 基础设施
   - 图片管理
   - flag 指令

13. 安全提醒  
   保留现有内容，压缩成 3~4 条高信号提示。

14. License  
   若仓库仍无 `LICENSE` 文件，则继续明确写“当前未提供 LICENSE”。

- 当前 README 中这些内容要弱化或重写：
  - 过长的“项目亮点”平铺列表
  - 过粗的目录树说明
  - 未覆盖 AI / OJ Task / Observability 的接口概览
  - 容易被理解成“已经完全平台化”的表述

## Public Interfaces / Docs Impact

- **不修改任何代码、接口、配置键或路由行为**。
- 只调整 `README.md` 的文档表达，文档中记录的接口、配置、依赖必须严格对应现有实现。
- README 对外口径固定为：
  - 单体应用
  - 业务型平台后端
  - AI 子域已落地主链路，但不是完整通用多 Agent 平台

## Test Plan

- 逐项核对 README 中提到的启动链与初始化顺序，依据 `cmd/main.go`、`internal/init/init.go`、`internal/router/router.go`。
- 核对 README 中列出的模块与服务职责，确保能在 `internal/service/system`、`internal/core`、`internal/router/system` 找到直接对应。
- 核对所有接口分组与示例路径，确保 AI、OJ Task、Observability、Health 等现有路由都被覆盖且不写错。
- 核对快速开始命令、CLI 参数、Docker 说明与 `.env.example`、`deploy/docker-compose.prod.yml`、`flag/*` 一致。
- 核对 docs 链接全部存在，避免 README 出现失效文档路径。
- 检查最终 README 是否满足两个验收标准：
  - 新读者 1 分钟内能看懂“项目做什么、架构怎么分、有什么亮点”
  - 面试场景下可以直接拿 README 作为项目讲稿的提纲

## Assumptions

- 受众默认是 **展示 + 面试**，不是纯维护手册。
- 改造方式默认是 **整体重写**，不是在原结构上打补丁。
- README 默认不新增徽章、截图、许可证声明或不存在的部署能力。
- 对 AI 能力的表述默认采取保守口径：只讲当前代码已落地的会话、SSE、interrupt/decision、Eino runtime 主链。
- 若后续进入执行阶段，先按仓库规则落盘计划到 `plan/cross-module/pending-readme-rewrite.md`，再实施文档修改。
