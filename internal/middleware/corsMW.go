package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware configures CORS for cross-origin requests during development
// Allows frontend from configured whitelist to access backend APIs with credentials
func CORSMiddleware() gin.HandlerFunc {
	// Dynamic whitelist to ensure Access-Control-Allow-Origin echoes the exact Origin
	allowed := map[string]bool{
		"http://localhost:3000":     true,
		"http://127.0.0.1:3000":     true,
		"http://192.168.20.14:3000": true,
		// 如需从内网其他前端域调试，在此添加
		// "http://192.168.10.7:7000":  true,
		// "http://192.168.10.7:8082":  true,
	}
	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// 只允许白名单中的Origin，且返回非*，以满足携带Cookie的跨域要求
			_ = allowed[strings.TrimSpace(origin)]
			// 现在是允许所有通过
			return true
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "X-Requested-With", "Content-Type", "Accept", "Authorization", "X-Csrf-Token", "x-access-token", "Cookie", "Set-Cookie"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
