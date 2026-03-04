package system

import (
	"strconv"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	serviceContract "personal_assistant/internal/service/contract"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ObservabilityCtrl 可观测性查询控制器：负责参数绑定/校验入口、调用 Service、统一返回结构。
type ObservabilityCtrl struct {
	observabilityService serviceContract.ObservabilityServiceContract
}

// QueryMetrics 查询聚合指标（metrics）。请求体为 JSON。
func (ctrl *ObservabilityCtrl) QueryMetrics(c *gin.Context) {
	var req request.ObservabilityMetricsQueryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("query observability metrics bind failed", zap.Error(err))
		response.BizFailWithCodeMsg(errors.CodeBindFailed, "参数绑定失败", c)
		return
	}
	data, err := ctrl.observabilityService.QueryMetrics(c.Request.Context(), &req)
	if err != nil {
		global.Log.Error("query observability metrics failed", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(data, c)
}

// QueryTraceByRequestID 按 request_id 查询 trace spans（分页参数见 parseTraceQueryPage）。
func (ctrl *ObservabilityCtrl) QueryTraceByRequestID(c *gin.Context) {
	requestID := strings.TrimSpace(c.Param("request_id"))
	limit, _, includePayload, includeErrorDetail := parseTraceQueryPage(c)
	data, err := ctrl.observabilityService.QueryTraceByRequestID(
		c.Request.Context(),
		requestID,
		limit,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		global.Log.Error("query trace by request id failed", zap.Error(err), zap.String("request_id", requestID))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(data, c)
}

// QueryTraceByTraceID 按 trace_id 查询 trace spans（支持 offset/limit 与是否包含 payload）。
func (ctrl *ObservabilityCtrl) QueryTraceByTraceID(c *gin.Context) {
	traceID := strings.TrimSpace(c.Param("trace_id"))
	limit, offset, includePayload, includeErrorDetail := parseTraceQueryPage(c)
	data, err := ctrl.observabilityService.QueryTraceByTraceID(
		c.Request.Context(),
		traceID,
		limit,
		offset,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		global.Log.Error("query trace by trace id failed", zap.Error(err), zap.String("trace_id", traceID))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(data, c)
}

// QueryTrace 按多条件查询 trace spans（JSON 请求体：trace_id/request_id/service/stage/status/time range 等）。
func (ctrl *ObservabilityCtrl) QueryTrace(c *gin.Context) {
	var req request.ObservabilityTraceQueryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("query observability trace bind failed", zap.Error(err))
		response.BizFailWithCodeMsg(errors.CodeBindFailed, "参数绑定失败", c)
		return
	}
	data, err := ctrl.observabilityService.QueryTrace(c.Request.Context(), &req)
	if err != nil {
		global.Log.Error("query observability trace failed", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(data, c)
}

// parseTraceQueryPage 解析 trace 查询分页参数（querystring）：
//   - limit：默认 200
//   - offset：默认 0
//   - include_payload：默认 true（是否返回 request/response 片段，建议前端按需关闭）
//   - include_error_detail：默认 false（是否返回 error_stack/error_detail_json）
func parseTraceQueryPage(c *gin.Context) (limit int, offset int, includePayload bool, includeErrorDetail bool) {
	limit = 200
	offset = 0
	includePayload = true
	includeErrorDetail = false

	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("include_payload")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			includePayload = parsed
		}
	}
	if raw := strings.TrimSpace(c.Query("include_error_detail")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			includeErrorDetail = parsed
		}
	}
	return
}
