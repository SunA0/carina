package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/auth"
	"github.com/suna0/carina/vanilla"
)

// JWTAuthMiddleware JWT 认证中间件。
// 从 AUTHORIZATION header / _jwt query param / _jwt cookie 中提取 token。
// 认证成功后将业务上下文注入 gin.Context。
func JWTAuthMiddleware(skipPaths []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过指定路径
		path := c.Request.URL.Path
		for _, skip := range skipPaths {
			if strings.HasPrefix(path, skip) {
				c.Next()
				return
			}
		}

		// 根路径跳过认证
		if path == "/" {
			c.Next()
			return
		}

		// 提取 JWT token
		jwtToken := extractToken(c)
		if jwtToken == "" {
			c.AbortWithStatusJSON(http.StatusOK, vanilla.MakeErrorResponse(500,
				"jwt:missing_token", "missing jwt token"))
			return
		}

		// 解析 token
		claims, err := auth.Decode(jwtToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusOK, vanilla.MakeErrorResponse(500,
				"jwt:invalid_jwt_token", fmt.Sprintf("invalid token: %v", err)))
			return
		}

		// 注入业务上下文
		if factory := vanilla.GetBusinessContextFactory(); factory != nil {
			bCtx := factory.NewContext(c.Request.Context(), claims.UserId, jwtToken)
			c.Request = c.Request.WithContext(bCtx)
		}

		// 注入 user info 到 gin.Context
		c.Set("user_id", claims.UserId)
		c.Set("auth_user_id", claims.AuthUserId)
		c.Set("jwt_token", jwtToken)

		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	// 1. AUTHORIZATION header
	token := c.GetHeader("AUTHORIZATION")
	if token != "" {
		return token
	}
	// 2. _jwt query param
	token = c.Query("_jwt")
	if token != "" {
		return token
	}
	// 3. _jwt cookie
	if cookie, err := c.Cookie("_jwt"); err == nil {
		return cookie
	}
	return ""
}
