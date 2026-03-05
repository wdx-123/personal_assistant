package errors

import (
	"errors"

	"gorm.io/gorm"

	"personal_assistant/pkg/jwt"
)

// JWTError 定义JWT错误类型
type JWTError struct {
	Code    BizCode
	Message string
	Err     error
}

func (e *JWTError) Error() string {
	return e.Message
}

// ClassifyJWTError 分类JWT相关错误并返回对应的状态码
func ClassifyJWTError(err error) *JWTError {
	switch {
	case errors.Is(err, jwt.TokenExpired):
		return &JWTError{
			Code:    CodeTokenExpired,
			Message: "Token已过期",
			Err:     err,
		}
	case errors.Is(err, jwt.TokenMalformed):
		return &JWTError{
			Code:    CodeTokenMalformed,
			Message: "Token格式错误",
			Err:     err,
		}
	case errors.Is(err, jwt.TokenInvalid), errors.Is(err, jwt.TokenNotValidYet):
		return &JWTError{
			Code:    CodeTokenInvalid,
			Message: "Token无效",
			Err:     err,
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		return &JWTError{
			Code:    CodeUserNotFound,
			Message: "用户不存在",
			Err:     err,
		}
	default:
		return &JWTError{
			Code:    CodeInternalError,
			Message: "服务器内部错误",
			Err:     err,
		}
	}
}
