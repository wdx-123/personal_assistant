package response

import (
	"net/http"
	"personal_assistant/global"
	"reflect"

	"github.com/gin-gonic/gin"
)

// Response 用于封装 API 响应结构，支持泛型转换和分页等功能
type Response[T any, R any] struct {
	Trans  Transformer[T, R]
	Ctx    *gin.Context
	Code   global.AppCode
	layout gin.H
}

func NewResponse[T any, R any](c *gin.Context) *Response[T, R] {
	return &Response[T, R]{Ctx: c}
}

// SetCode 设置响应体
func (r *Response[T, R]) SetCode(code global.AppCode) *Response[T, R] {
	r.Code = code
	return r
}

// SetTrans 设置数据转换器（Transformer），用于结构体转换。
func (r *Response[T, R]) SetTrans(t Transformer[T, R]) *Response[T, R] {
	r.Trans = t
	return r
}

func (r *Response[T, R]) getCode() global.AppCode {
	if r.Code == 0 {
		r.Code = global.StatusOK
	}
	return r.Code
}

// Success 返回带自定义消息的成功响应
func (r *Response[T, R]) Success(msg string, data any) {
	r.buildLayout(data)
	r.withMessage(msg)
	r.Ctx.JSON(http.StatusOK, r.layout)
}

// Failed 返回带错误信息的失败响应
func (r *Response[T, R]) Failed(err string, data any) {
	if r.Code == 0 {
		r.SetCode(global.StatusInternalServerError)
	}
	r.buildLayout(data)
	r.withError(err)
	r.Ctx.JSON(http.StatusOK, r.layout)
}

// Item 返回单条数据的标准响应。
func (r *Response[T, R]) Item(data T) {
	r.buildLayout(data)
	r.Ctx.JSON(http.StatusOK, r.layout)
}

func (r *Response[T, R]) buildLayout(data any) *Response[T, R] {
	r.layout = gin.H{
		"code": r.getCode(),
		"data": r.response(data),
	}
	return r
}

func (r *Response[T, R]) response(data any) any {
	if data == nil {
		return data
	}
	switch reflect.TypeOf(data).Kind() {
	case reflect.Slice:
		// todo
		return nil
	case reflect.Struct:
		// 确保 data 是 T 类型
		if item, ok := data.(T); ok {
			return r.responseItem(&item)
		}
		return data
	default:
		return data
	}

}

func (r *Response[T, R]) responseItem(data *T) any {
	if r.Trans == nil {
		return data
	}
	return r.Trans.ToResponse(data)
}

func (r *Response[T, R]) withMessage(msg string) *Response[T, R] {
	r.layout["messages"] = msg
	return r
}
func (r *Response[T, R]) withError(err string) *Response[T, R] {
	r.layout["error"] = err
	return r
}
