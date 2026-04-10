//go:build windows
// +build windows

package core

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// initServer 函数初始化一个标准的 HTTP 服务器（适用于 Windows 系统）
func initServer(address string, router *gin.Engine) server {
	// 实现了server内部的方法，listenAndServer
	return &http.Server{
		Addr:           address,          // 设置服务器监听的地址
		Handler:        router,           // 设置请求处理器（路由）
		ReadTimeout:    10 * time.Minute, // 设置请求的读取超时时间为 10 分钟
		WriteTimeout:   0,                // SSE 通过单次写 deadline 控制，不使用全局写超时
		MaxHeaderBytes: 1 << 20,          // 设置最大请求头的大小（1MB）
	}
}

/*
ReadTimeout 的作用：
- ✅ 正常情况 ：100秒 < 10分钟(600秒)，上传成功
- ❌ 异常情况 ：如果网络极慢或恶意攻击，超过10分钟还没读完，服务器会主动断开连接
WriteTimeout 的作用：
- ✅ 正常情况 ：客户端网络正常，50MB数据在10分钟内发送完成
- ❌ 异常情况 ：客户端网络极慢或故意不接收数据，超过10分钟服务器主动断开
*/
