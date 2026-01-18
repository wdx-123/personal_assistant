package middleware

import (
	"context"
	"errors"
	"time"

	"personal_assistant/global"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
)

// TimeoutMiddleware 请求超时中间件
// timeout: 超时时间，如果为0则使用默认的30秒
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	if timeout == 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}

	return func(c *gin.Context) {
		// 创建带超时的context
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// 替换请求的context
		c.Request = c.Request.WithContext(ctx)

		// 创建一个channel来接收处理完成的信号
		finished := make(chan struct{})

		go func() {
			defer func() {
				if r := recover(); r != nil {
					// 处理panic
					global.Log.Error("Request panic recovered in timeout middleware")
				}
				close(finished)
			}()
			c.Next()
		}()

		// 等待请求完成或超时
		select {
		case <-finished:
			// 请求正常完成
			return
		case <-ctx.Done():
			// 请求超时
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				c.Header("Connection", "close")
				response.NewResponse[resp.ErrorResponse, resp.ErrorResponse](c).
					SetCode(global.StatusInternalServerError).
					Failed("Request timeout", &resp.ErrorResponse{
						Message: "请求超时，请稍后重试",
						Code:    int(global.StatusInternalServerError),
					})
				c.Abort()
				return
			}
		}
	}
}
