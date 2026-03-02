package response

// ErrorResponse 通用错误响应结构体
type ErrorResponse struct {
	Message string `json:"message"` // 错误消息
	Code    int    `json:"code"`    // 错误代码
}

func (e ErrorResponse) ToResponse(input *ErrorResponse) *ErrorResponse {
	return input
}
