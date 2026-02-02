package errors

import (
	"fmt"
)

// BizError 业务错误类型
// Service 层使用，封装业务错误码和自定义消息
// Controller 层通过 response.FailWithBizError 返回给前端
type BizError struct {
	Code    BizCode // 业务错误码
	Message string  // 自定义错误消息（展示给用户）
	Cause   error   // 原始错误（仅用于日志，不返回前端）
}

// Error 实现 error 接口
func (e *BizError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持 errors.Is 和 errors.As
func (e *BizError) Unwrap() error {
	return e.Cause
}

// ==================== 创建 BizError 的快捷方法 ====================

// New 创建业务错误（使用默认消息）
func New(code BizCode) *BizError {
	return &BizError{
		Code:    code,
		Message: code.Message(),
	}
}

// NewWithMsg 创建业务错误（自定义消息）
func NewWithMsg(code BizCode, message string) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装原始错误（使用默认消息）
func Wrap(code BizCode, cause error) *BizError {
	return &BizError{
		Code:    code,
		Message: code.Message(),
		Cause:   cause,
	}
}

// WrapWithMsg 包装原始错误（自定义消息）
func WrapWithMsg(code BizCode, message string, cause error) *BizError {
	return &BizError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// ==================== 辅助函数 ====================

// IsBizError 判断是否为业务错误
func IsBizError(err error) bool {
	_, ok := err.(*BizError)
	return ok
}

// FromError 从 error 中提取 BizError，如果不是则返回 nil
func FromError(err error) *BizError {
	if err == nil {
		return nil
	}
	if bizErr, ok := err.(*BizError); ok {
		return bizErr
	}
	return nil
}
