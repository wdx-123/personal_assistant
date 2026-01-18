package core

import (
    "net/http"
    "personal_assistant/global"
    "personal_assistant/internal/router"

    "go.uber.org/zap"
)

type server interface {
	ListenAndServe() error
}

func RunServer() {
    addr := global.Config.System.Addr()
    Router := router.InitRouter()

    // 初始化服务器并启动
    s := initServer(addr, Router)
    global.Log.Info("server starting", zap.String("address", addr))
    if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        global.Log.Error("server start failed", zap.Error(err))
    }
}
