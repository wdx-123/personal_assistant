package response

import (
	"personal_assistant/global"
	erro "personal_assistant/pkg/errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// APIHelper API辅助工具
type APIHelper struct {
	ctx     *gin.Context
	logger  *zap.Logger
	apiName string
}

// NewAPIHelper 创建API辅助工具
func NewAPIHelper(c *gin.Context, apiName string) *APIHelper {
	return &APIHelper{
		ctx:     c,
		logger:  global.Log,
		apiName: apiName,
	}
}

// HandleBindError 处理参数绑定错误
func (h *APIHelper) HandleBindError(err error) {
	h.CommonError(global.ErrBindDataFailed, global.StatusBadRequest, err)
}

// HandleJWTError 处理JWT相关错误
func (h *APIHelper) HandleJWTError(jwtErr *erro.JWTError) {
	h.CommonError(jwtErr.Message, jwtErr.Code, jwtErr.Err)
}

// CommonError 通用错误处理
func (h *APIHelper) CommonError(message string, code global.AppCode, err error) {
	// 构建日志字段
	logFields := []zap.Field{
		zap.String("message", message),
		zap.Int("code", int(code)),
	}
	// 只有当 err 不为 nil 时才添加错误字段
	if err != nil {
		logFields = append(logFields, zap.Error(err))
	}
	// 记录错误日志
	h.logger.Error("[Api]"+h.apiName+" failed", logFields...)
}
