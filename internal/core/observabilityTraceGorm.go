package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"personal_assistant/global"
	obstrace "personal_assistant/pkg/observability/trace"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	// traceSpanCreateKey 等常量是 GORM 实例中用于存储 Span 对象的 Key。
	// GORM 的 WithContext 设计允许我们在 db 实例中传递上下文信息。
	// 这里的 Key 用于在 Before 和 After 钩子之间传递 *obstrace.SpanEvent 对象。
	traceSpanCreateKey = "obs_trace_span_create"
	traceSpanQueryKey  = "obs_trace_span_query"
	traceSpanUpdateKey = "obs_trace_span_update"
	traceSpanDeleteKey = "obs_trace_span_delete"
	traceSpanRawKey    = "obs_trace_span_raw"
	traceSpanRowKey    = "obs_trace_span_row"
)

// registerGORMTraceCallbacks 为 GORM DB 实例注册全链路追踪钩子（Callbacks）。
//
// 原理：
//   - 利用 GORM 的 Plugin 机制，在 CRUD 操作的 Before 和 After 阶段插入追踪逻辑。
//   - Before 阶段：启动 Span，记录操作类型和表名。
//   - After 阶段：结束 Span，记录耗时、错误信息和影响行数。
//
// 覆盖范围：
//   - Create (INSERT)
//   - Query (SELECT)
//   - Update (UPDATE)
//   - Delete (DELETE)
//   - Raw (原生 SQL)
//   - Row (原生 SQL 查询单行)
//
// 注意：
//   - 必须在 GORM 初始化后立即调用。
//   - 依赖 global.ObservabilityTraces 后端，若未初始化则自动跳过。
func registerGORMTraceCallbacks(db *gorm.DB, serviceName string) {
	if db == nil {
		return
	}
	// 注册 Create 操作的追踪钩子
	registerCallback(db.Callback().Create().Before("gorm:create"), "obs_trace:before_create", gormTraceBefore("create", traceSpanCreateKey, serviceName))
	registerCallback(db.Callback().Create().After("gorm:create"), "obs_trace:after_create", gormTraceAfter(traceSpanCreateKey, "create"))

	// 注册 Query 操作的追踪钩子
	registerCallback(db.Callback().Query().Before("gorm:query"), "obs_trace:before_query", gormTraceBefore("query", traceSpanQueryKey, serviceName))
	registerCallback(db.Callback().Query().After("gorm:query"), "obs_trace:after_query", gormTraceAfter(traceSpanQueryKey, "query"))

	// 注册 Update 操作的追踪钩子
	registerCallback(db.Callback().Update().Before("gorm:update"), "obs_trace:before_update", gormTraceBefore("update", traceSpanUpdateKey, serviceName))
	registerCallback(db.Callback().Update().After("gorm:update"), "obs_trace:after_update", gormTraceAfter(traceSpanUpdateKey, "update"))

	// 注册 Delete 操作的追踪钩子
	registerCallback(db.Callback().Delete().Before("gorm:delete"), "obs_trace:before_delete", gormTraceBefore("delete", traceSpanDeleteKey, serviceName))
	registerCallback(db.Callback().Delete().After("gorm:delete"), "obs_trace:after_delete", gormTraceAfter(traceSpanDeleteKey, "delete"))

	// 注册 Raw SQL 操作的追踪钩子
	registerCallback(db.Callback().Raw().Before("gorm:raw"), "obs_trace:before_raw", gormTraceBefore("raw", traceSpanRawKey, serviceName))
	registerCallback(db.Callback().Raw().After("gorm:raw"), "obs_trace:after_raw", gormTraceAfter(traceSpanRawKey, "raw"))

	// 注册 Row 操作的追踪钩子
	registerCallback(db.Callback().Row().Before("gorm:row"), "obs_trace:before_row", gormTraceBefore("row", traceSpanRowKey, serviceName))
	registerCallback(db.Callback().Row().After("gorm:row"), "obs_trace:after_row", gormTraceAfter(traceSpanRowKey, "row"))
}

// registerCallback 辅助函数：安全地注册 GORM 回调。
// 捕获注册过程中的错误并记录 Debug 日志，防止因重复注册等原因导致 Panic。
func registerCallback(p interface {
	Register(string, func(*gorm.DB)) error
}, name string, fn func(*gorm.DB)) {
	if p == nil || fn == nil {
		return
	}
	if err := p.Register(name, fn); err != nil && global.Log != nil {
		global.Log.Debug("register gorm trace callback skipped", zap.String("name", name), zap.Error(err))
	}
}

// gormTraceBefore 创建“操作前”的回调函数。
//
// 职责：
//  1. 检查追踪是否启用。
//  2. 从 Context 中提取父 Span（如果有）。
//  3. 启动一个新的 Span（名称如 "db.create"）。
//  4. 将 Span 实例存入 GORM 的 db.Instance，供 After 阶段使用。
func gormTraceBefore(op string, key string, serviceName string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		// 防御性检查：DB 实例、Context 或追踪后端未就绪时不执行
		if tx == nil || tx.Statement == nil || global.ObservabilityTraces == nil {
			return
		}

		// 从 GORM Statement 中获取上下文，并启动新的 Span
		ctx := tx.Statement.Context
		ctx, span := obstrace.StartSpan(ctx, obstrace.StartOptions{
			Service: serviceName,
			Stage:   "repository.db", // 统一阶段标识
			Name:    "db." + op,      // 操作名称，如 db.query
			Kind:    "client",        // DB 也是一种 Client 调用
			Tags: map[string]string{
				"table":     strings.TrimSpace(tx.Statement.Table),
				"op":        op,
				"db.system": "mysql", // 标记数据库类型
			},
		})

		// 将更新后的 Context 写回 Statement，以便下游能获取到最新的 Trace 信息
		tx.Statement.Context = ctx
		// 将 SpanEvent 对象存入 Instance，以便在 After 回调中取出
		_ = tx.InstanceSet(key, span)
	}
}

// gormTraceAfter 创建“操作后”的回调函数。
//
// 职责：
//  1. 取出 Before 阶段创建的 Span。
//  2. 检查执行错误（tx.Error）。
//  3. 记录错误详情、影响行数等元数据。
//  4. 结束 Span 并异步上报。
func gormTraceAfter(key string, op string) func(*gorm.DB) {
	return func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || global.ObservabilityTraces == nil {
			return
		}
		// 取出之前存储的 Span
		v, ok := tx.InstanceGet(key)
		if !ok {
			return
		}
		spanEvent, ok := v.(*obstrace.SpanEvent)
		if !ok || spanEvent == nil {
			return
		}

		status := obstrace.SpanStatusOK
		errorCode := ""
		message := ""

		// 错误处理逻辑
		if tx.Error != nil {
			status = obstrace.SpanStatusError
			errorCode = "db_error"
			message = tx.Error.Error()
			// 构建详细的错误上下文（包含操作类型、表名、具体错误）
			if detail := buildDBErrorDetail(op, tx.Statement.Table, tx.Error); detail != "" {
				spanEvent.WithErrorDetail(detail)
			}
		}

		// 结束 Span，追加最终统计信息
		span := spanEvent.End(status, errorCode, message, map[string]string{
			"table":         strings.TrimSpace(tx.Statement.Table),
			"op":            op,
			"rows_affected": fmt.Sprintf("%d", tx.RowsAffected),
		})

		// 异步上报 Span
		if span != nil {
			if err := global.ObservabilityTraces.RecordSpan(tx.Statement.Context, span); err != nil && global.Log != nil {
				global.Log.Error("record db trace span failed", zap.Error(err))
			}
		}
	}
}

// buildDBErrorDetail 构建数据库错误的结构化详情 JSON。
// 包含：操作类型 (op)、表名 (table)、错误信息 (error)。
func buildDBErrorDetail(op, table string, err error) string {
	payload := map[string]string{
		"op":    strings.TrimSpace(op),
		"table": strings.TrimSpace(table),
	}
	if err != nil {
		payload["error"] = strings.TrimSpace(err.Error())
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return ""
	}
	return string(data)
}
