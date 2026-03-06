package system

import (
	"encoding/json"
	stdErrors "errors"
	"io"
	"strconv"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	serviceContract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
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
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", c)
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

// QueryRuntimeMetrics 查询后台执行器运行时指标。
func (ctrl *ObservabilityCtrl) QueryRuntimeMetrics(c *gin.Context) {
	var req request.ObservabilityRuntimeMetricQueryReq
	if err := bindJSONStrict(c, &req); err != nil {
		global.Log.Error("query observability runtime bind failed", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", c)
		return
	}
	data, err := ctrl.observabilityService.QueryRuntimeMetrics(c.Request.Context(), &req)
	if err != nil {
		global.Log.Error("query observability runtime failed", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(data, c)
}

// QueryTraceDetail 统一按 id + id_type 查询 trace spans 详情。
func (ctrl *ObservabilityCtrl) QueryTraceDetail(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	idType := request.NormalizeTraceDetailIDType(c.Query("id_type"))
	limit, offset, includePayload, includeErrorDetail := parseTraceQueryPage(c)

	data, err := ctrl.observabilityService.QueryTraceDetail(
		c.Request.Context(),
		id,
		idType,
		limit,
		offset,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		global.Log.Error(
			"query trace detail failed",
			zap.Error(err),
			zap.String("id", id),
			zap.String("id_type", idType),
		)
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(data, c)
}

// QueryTrace 按条件查询 root 摘要（JSON 请求体：trace_id/request_id/service/status/root_stage/time range）。
func (ctrl *ObservabilityCtrl) QueryTrace(c *gin.Context) {
	var req request.ObservabilityTraceQueryReq
	if err := bindJSONStrict(c, &req); err != nil {
		global.Log.Error("query observability trace bind failed", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", c)
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

// parseTraceQueryPage 解析 trace 详情查询分页参数（querystring）：
//   - limit：默认 200
//   - offset：默认 0
//   - include_payload：默认 true（是否返回 request/response 片段）
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

// bindJSONStrict 严格绑定 JSON：拒绝未知字段，避免旧参数混入新契约。
func bindJSONStrict(c *gin.Context, target any) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return io.EOF
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return stdErrors.New("unexpected trailing json content")
		}
		return err
	}
	return nil
}
