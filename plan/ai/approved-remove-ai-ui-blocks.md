# 目标

全局移除 AI 消息中的 UI Blocks / A2UI 持久化与接口协议，删除 `UIBlocksJSON = "[]"` 相关逻辑，并让后端不再读写或返回 `ui_blocks`。

# 范围

- AI 消息实体：`internal/model/entity/ai.go`
- AI 响应 DTO：`internal/model/dto/response/aiResp.go`
- AI 消息创建、投影、映射：`internal/service/system/aiSvc.go`、`aiProjector.go`、`aiMapper.go`
- AI sink/projector 测试：`internal/service/system/aiSink_test.go`、`aiProjector_test.go`
- 自动迁移：`flag/flagSql.go`

# 改动

1. 删除实体字段：
   - 从 `entity.AIMessage` 移除 `UIBlocksJSON`。
   - 后端不再映射数据库列 `ui_blocks_json`。

2. 删除响应协议：
   - 从 `AssistantMessageResp` 移除 `UIBlocks` / `json:"ui_blocks"`。
   - 删除 `AssistantA2UIBinding`、`AssistantA2UIComponent`、`AssistantA2UISurface`、`AssistantA2UIBlock`。
   - 删除 `AssistantStructuredBlockPayload.UIBlock` 相关字段；若该 payload 只剩 scope，则改名或收缩为仅 scope 结构。

3. 删除业务读写：
   - 移除创建消息时的 `UIBlocksJSON: "[]"`。
   - 移除 projector 落库时的 `p.message.UIBlocksJSON = "[]"`。
   - 移除 mapper 中 `decodeAssistantUIBlocks` 及相关调用。

4. 删除测试残留：
   - 移除测试数据里的 `UIBlocksJSON` 初始化。
   - 移除断言 `UIBlocksJSON == "[]"`。

5. 数据库迁移：
   - 在 `flag.SQL()` 中增加幂等迁移函数，如 `dropAIMessageUIBlocksColumn(db)`。
   - 若 `ai_messages.ui_blocks_json` 存在，则执行 `DROP COLUMN ui_blocks_json`。
   - 该迁移只处理 `ui_blocks_json`，不影响 `trace_items_json` 和 `scope_json`。

# 验证

- 执行 `go test ./internal/service/system -run "AIMessageProjector|AIStreamSink|AIService"`.
- 执行 `go test ./internal/model/... ./internal/repository/... ./internal/service/system/...`，若耗时过长则至少覆盖 AI 相关包。
- 执行 `go test ./...`，如环境依赖导致失败，记录具体失败原因。
- 使用 `rg "UIBlocksJSON|ui_blocks_json|UIBlocks|AssistantA2UI|A2UI"` 确认无实现残留；计划历史文件中的说明不作为阻塞项。

# 风险

- 这是破坏性协议变更：前端如果仍读取 `message.ui_blocks`，需要同步删除或兼容缺失字段。
- 这是破坏性数据库变更：`DROP COLUMN ui_blocks_json` 会永久删除历史 UI block 数据。
- GORM `AutoMigrate` 不会自动删列，所以必须显式迁移；生产执行前应确认该字段没有保留价值。
- 如果后续恢复结构化 UI，需要重新设计独立协议或重新建表/字段。

# 执行顺序

1. 将本计划改名为 `approved-remove-ai-ui-blocks.md`。
2. 删除 DTO 层 A2UI/UIBlocks 类型与字段。
3. 删除实体字段和 service 层读写映射。
4. 更新 projector/sink 相关测试。
5. 增加 `ai_messages.ui_blocks_json` 幂等删列迁移。
6. 运行验证命令并清理编译错误。
7. 汇报改动范围、迁移影响和测试结果。

# 待确认

- 请确认是否按本计划执行，包括物理删除数据库列 `ai_messages.ui_blocks_json`。
