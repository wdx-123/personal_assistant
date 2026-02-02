package response

import (
	"net/http"
	"time"

	"personal_assistant/pkg/errors"

	"github.com/gin-gonic/gin"
)

// BizResponse 统一响应结构（新版）
// 用于新功能开发，与原有 Response 结构兼容
type BizResponse struct {
	Code      int    `json:"code"`      // 业务状态码：0成功，其他失败
	Success   bool   `json:"success"`   // 是否成功
	Message   string `json:"message"`   // 响应消息
	Data      any    `json:"data"`      // 响应数据
	Timestamp int64  `json:"timestamp"` // 响应时间戳（毫秒）
}

// PageData 分页数据结构
type PageData struct {
	List     any   `json:"list"`      // 数据列表
	Total    int64 `json:"total"`     // 总数
	Page     int   `json:"page"`      // 当前页码
	PageSize int   `json:"page_size"` // 每页大小
}

// ==================== 核心响应函数 ====================

// BizResult 构建响应并返回
func BizResult(code errors.BizCode, data any, message string, c *gin.Context) {
	c.JSON(http.StatusOK, BizResponse{
		Code:      code.Int(),
		Success:   code == errors.CodeSuccess,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})
}

// ==================== 成功响应 ====================

// BizOk 成功响应（无数据）
func BizOk(c *gin.Context) {
	BizResult(errors.CodeSuccess, nil, "操作成功", c)
}

// BizOkWithMessage 成功响应（自定义消息）
func BizOkWithMessage(message string, c *gin.Context) {
	BizResult(errors.CodeSuccess, nil, message, c)
}

// BizOkWithData 成功响应（带数据）
func BizOkWithData(data any, c *gin.Context) {
	BizResult(errors.CodeSuccess, data, "操作成功", c)
}

// BizOkWithDetailed 成功响应（带数据和消息）
func BizOkWithDetailed(data any, message string, c *gin.Context) {
	BizResult(errors.CodeSuccess, data, message, c)
}

// BizOkWithPage 成功响应（分页数据）
func BizOkWithPage(list any, total int64, page, pageSize int, c *gin.Context) {
	BizResult(errors.CodeSuccess, PageData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, "获取成功", c)
}

// ==================== 失败响应 ====================

// BizFail 失败响应（默认内部错误）
func BizFail(c *gin.Context) {
	BizResult(errors.CodeInternalError, nil, errors.CodeInternalError.Message(), c)
}

// BizFailWithMessage 失败响应（自定义消息，默认内部错误码）
func BizFailWithMessage(message string, c *gin.Context) {
	BizResult(errors.CodeInternalError, nil, message, c)
}

// BizFailWithCode 失败响应（指定错误码，使用默认消息）
func BizFailWithCode(code errors.BizCode, c *gin.Context) {
	BizResult(code, nil, code.Message(), c)
}

// BizFailWithCodeMsg 失败响应（指定错误码和自定义消息）
func BizFailWithCodeMsg(code errors.BizCode, message string, c *gin.Context) {
	BizResult(code, nil, message, c)
}

// BizFailWithError 失败响应（从 BizError 中提取错误码和消息）
// Controller 层处理 Service 层返回的 BizError 的主要方法
func BizFailWithError(err error, c *gin.Context) {
	if bizErr := errors.FromError(err); bizErr != nil {
		BizResult(bizErr.Code, nil, bizErr.Message, c)
		return
	}
	// 非 BizError，返回通用内部错误（不暴露原始错误信息）
	BizResult(errors.CodeInternalError, nil, errors.CodeInternalError.Message(), c)
}

// BizFailWithErrorData 失败响应（从 BizError 提取，并附带数据）
func BizFailWithErrorData(err error, data any, c *gin.Context) {
	if bizErr := errors.FromError(err); bizErr != nil {
		BizResult(bizErr.Code, data, bizErr.Message, c)
		return
	}
	BizResult(errors.CodeInternalError, data, errors.CodeInternalError.Message(), c)
}

// ==================== 特殊响应 ====================

// BizNoAuth 未授权响应（HTTP 401）
func BizNoAuth(message string, c *gin.Context) {
	c.JSON(http.StatusUnauthorized, BizResponse{
		Code:      errors.CodeUnauthorized.Int(),
		Success:   false,
		Message:   message,
		Data:      nil,
		Timestamp: time.Now().UnixMilli(),
	})
}

// BizNoAuthWithReload 未授权响应（需要前端刷新）
func BizNoAuthWithReload(message string, c *gin.Context) {
	c.JSON(http.StatusUnauthorized, BizResponse{
		Code:      errors.CodeUnauthorized.Int(),
		Success:   false,
		Message:   message,
		Data:      gin.H{"reload": true},
		Timestamp: time.Now().UnixMilli(),
	})
}
