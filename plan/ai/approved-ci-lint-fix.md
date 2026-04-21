# 目标

修复当前本地复现 CI 时暴露的 `golangci-lint` 失败问题，让仓库可以通过 `.github/workflows/ci.yml` 中的本地可复现检查。

# 范围

仅处理本次 CI 检测明确暴露的问题：

- 删除 AI 最小流式重构后遗留的未使用函数。
- 对 lint 报出的格式问题文件执行 `gofmt`。
- 不新增功能，不调整 API，不改数据库结构，不恢复已删除的 plan/tool/interrupt/resume/A2UI 链路。

# 改动

计划修改以下文件：

- `internal/service/system/aiMapper.go`
  - 删除未使用的 `encodeJSON`。
  - 删除未使用的 `splitReplyChunks`。
  - 删除未使用的 `buildScopeInfo`。
- `internal/service/system/aiSink.go`
  - 删除未使用的 `(*aiStreamSink).applyEvent`。
- `internal/infrastructure/sse/interfaces.go`
  - 执行 `gofmt`。
- `internal/infrastructure/sse/types.go`
  - 执行 `gofmt`。
- `internal/middleware/corsMW.go`
  - 执行 `gofmt`。
- `pkg/casbin/casbin.go`
  - 执行 `gofmt`。

# 验证

按 CI 顺序复跑：

```powershell
go mod download
& 'C:\Program Files\Git\usr\bin\bash.exe' scripts/check_no_legacy_error_tracking.sh
go test ./...
go vet ./...
golangci-lint run --timeout=5m
```

# 风险

- 删除项都是 `unused` 报出的未引用函数，风险较低。
- `gofmt` 会产生格式化 diff，可能触碰到非 AI 文件，但只限 lint 已报告的格式问题文件。

# 执行顺序

1. 将本计划从 `pending` 改名为 `approved`。
2. 删除 AI 相关未使用函数。
3. 对 lint 报出的格式文件执行 `gofmt`。
4. 复跑 CI 本地命令。
5. 汇总结果。

# 待确认

请确认是否按本计划执行。
