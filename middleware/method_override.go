package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// MethodOverrideMiddleware 支持 _method 参数重写 HTTP Method。
// 用于 REST 兼容（HTML form 只支持 GET/POST）。
func MethodOverrideMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "POST" {
			if method := c.Query("_method"); method != "" {
				c.Request.Method = strings.ToUpper(method)
			}
		}
		c.Next()
	}
}
