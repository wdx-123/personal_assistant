# 目标

- 盘点项目当前通过环境变量读取的配置项。
- 补齐 `.env.example` 中缺失的模板变量，重点覆盖 AI、认证密钥、数据库连接补充项和本地开发必需项。
- 如当前工作区不存在 `.env`，基于模板生成一份仅含占位值和注释的本地开发模板，方便后续手工填写。

# 范围

- `./.env.example`
- `./.env`（仅在用户确认后创建或更新模板，不写入真实生产密钥）
- 只读参考：
  - `internal/core/config.go`
  - `internal/model/config/ai.go`
  - `README.md`

# 改动

- 对照 `internal/core/config.go` 中已绑定的环境变量，补齐 `.env.example` 缺失项。
- 新增 AI 配置模板，至少包括：
  - `AI_PROVIDER`
  - `AI_API_KEY`
  - `AI_BASE_URL`
  - `AI_MODEL`
  - `AI_BY_AZURE`
  - `AI_API_VERSION`
  - `AI_SYSTEM_PROMPT`
  - `AI_TEMPERATURE`
  - `AI_MAX_COMPLETION_TOKENS`
- 补齐当前模板中容易遗漏但会影响启动或行为的基础配置项，视实际绑定情况纳入：
  - `DB_CONFIG`
  - `DB_MAX_IDLE_CONNS`
  - `DB_MAX_OPEN_CONNS`
  - `DB_LOG_MODE`
  - `JWT_ACCESS_TOKEN_EXPIRY_TIME`
  - `JWT_REFRESH_TOKEN_EXPIRY_TIME`
  - `JWT_ISSUER`
  - `REDIS_ACTIVE_USER_STATE_TTL_SECONDS`
  - `REDIS_ACTIVE_USER_STATE_TTL_JITTER_SECONDS`
  - `STORAGE_LOCAL_BASE_URL`
  - `STORAGE_LOCAL_KEY_PREFIX`
  - `STORAGE_QINIU_KEY_PREFIX`
- 在模板注释中区分：
  - 本地启动必填
  - 使用特定能力时必填
  - 可保持默认
- 若创建本地 `.env`，默认写入可运行的开发占位模板：
  - 数据库和 Redis 指向本机
  - 存储驱动默认使用 `local`
  - AI 默认给出推荐 Provider 与 BaseURL 占位，但 `AI_API_KEY` 保留空值待用户填写
  - JWT / Session 写入明显标识为“仅本地开发使用”的占位密钥

# 验证

- 对照 `internal/core/config.go` 中 `BindEnv` 项，确认新增模板键名与代码一致。
- 检查 `.env.example` 分组结构是否清晰，避免重复或互相冲突的键。
- 若创建 `.env`，执行一次只读检查，确认关键启动项均已存在：
  - MySQL
  - Redis
  - JWT
  - Session
  - AI
  - Storage

# 风险

- 若直接写入真实密钥，存在泄露风险；本次仅应写占位值或本地开发临时值。
- 若把可选能力配置误标为启动必填，会增加本地启动门槛。
- 若遗漏 `storage.current` 与 AI 相关默认值，用户后续仍可能在存储或 AI 初始化阶段踩坑。

# 执行顺序

1. 整理代码中已绑定的环境变量清单，并按模块分组。
2. 设计 `.env.example` 的补齐方案，优先覆盖本地启动必需项和 AI 配置。
3. 更新 `.env.example` 注释与占位值。
4. 如用户需要，同时创建或更新本地 `.env` 模板。
5. 做键名与分组校验，并向用户汇总哪些值仍需其手工填写。

# 待确认

- 是否只更新 `.env.example`，还是同时创建一份本地 `.env` 模板。
- 本地 AI 模板是否默认按 `qwen + DashScope` 填推荐值，还是保留通用 OpenAI 兼容占位。
- JWT / Session 是否允许我直接写入本地开发临时随机值，还是统一保留占位字符串。
