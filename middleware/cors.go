package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware 创建可配白名单的 CORS 中间件。
// origins 为空或 nil 时允许所有源。
func CORSMiddleware(origins []string) gin.HandlerFunc {
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	return cors.New(cors.Config{
		AllowOrigins:           origins,
		AllowMethods:           []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:           []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Requested-With", "Request-Mode"},
		ExposeHeaders:          []string{"Content-Length"},
		AllowCredentials:       true,
		MaxAge:                 12 * time.Hour,
		AllowWildcard:          true,
		AllowBrowserExtensions: true,
		AllowWebSockets:        true,
		AllowOriginFunc: func(origin string) bool {
			if len(origins) == 1 && origins[0] == "*" {
				return true
			}
			for _, o := range origins {
				if o == origin {
					return true
				}
			}
			return false
		},
	})
}

// CORSMiddlewareWithOrigins 显式指定 CORS 白名单的便捷方法
func CORSMiddlewareWithOrigins(origins ...string) gin.HandlerFunc {
	return CORSMiddleware(origins)
}

// 确保 http 包被引用（gin-contrib/cors 需要）
var _ = http.StatusOK
