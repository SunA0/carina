package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/tracing"
)

// TracingMiddleware OpenTelemetry 链路追踪中间件
func TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		operationName := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		ctx, span := tracing.StartSpan(c.Request.Context(), operationName)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
