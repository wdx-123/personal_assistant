package response

// Package response 提供统一的 API 响应结构和数据转换工具。

// Transformer 接口
// 用于批量转换
// T: 输入类型，R: 输出类型
type Transformer[T any, R any] interface {
	ToResponse(input *T) *R
}
