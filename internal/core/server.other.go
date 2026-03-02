//go:build !windows
// +build !windows

package core

import (
	"time"

	"github.com/fvbock/endless"
	"github.com/gin-gonic/gin"
)

// initServer 函数初始化一个 Endless 服务器（适用于非 Windows 系统）
func initServer(address string, router *gin.Engine) server {
	// 实现优雅重启 “重启”
	s := endless.NewServer(address, router) // 使用 endless 包创建一个新的 HTTP 服务器实例
	s.ReadHeaderTimeout = 10 * time.Minute  // 设置请求头的读取超时时间为 10 分钟
	s.WriteTimeout = 10 * time.Minute       // 设置响应写入的超时时间为 10 分钟
	s.MaxHeaderBytes = 1 << 20              // 设置最大请求头的大小（1MB）

	return s // 返回创建的服务器实例
}

/*
优雅重启：endless
重点是重启：
	1. 启动新的服务器进程
	2. 新进程开始监听同一个端口
	3. 停止接受新的连接请求
	4. 等待现有连接完成处理
	5. 关闭旧进程
*/
/*
普通重启：
	1. 强制关闭服务器
	2. 断开所有连接（包括正在处理的请求）
	3. 重新启动服务器
	4. 重新监听端口
*/
