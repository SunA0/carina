package middleware

import (
	"github.com/suna0/carina/vanilla"

	"github.com/gin-gonic/gin"
)

// RecoveryMiddleware 包装 vanilla.RecoverPanic
func RecoveryMiddleware() gin.HandlerFunc {
	return vanilla.RecoverPanic()
}
