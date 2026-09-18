package vanilla

import (
	"fmt"
	"log"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/db"
	"github.com/suna0/carina/metrics"
)

// RecoverPanic Gin Recovery 中间件，捕获 panic 并统一处理：
//  1. 回滚事务（若存在）
//  2. 记录 metrics
//  3. 返回统一错误响应
func RecoverPanic() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 回滚事务
				if c.Request != nil && c.Request.Context() != nil {
					_ = db.RollbackTx(c.Request.Context())
				}

				// 记录 metrics
				if be, ok := err.(*BusinessError); ok {
					if be.IsPanicError() {
						if metrics.PanicCounter != nil {
							metrics.PanicCounter.Inc()
						}
					} else {
						if metrics.BusinessErrorCounter != nil {
							metrics.BusinessErrorCounter.Inc()
						}
					}
				} else {
					if metrics.PanicCounter != nil {
						metrics.PanicCounter.Inc()
					}
				}

				// 日志
				logPanic(c, err)

				// 返回错误响应
				if be, ok := err.(*BusinessError); ok {
					c.JSON(http.StatusOK, &Response{
						Code:    500,
						ErrCode: be.ErrCode,
						ErrMsg:  be.ErrMsg,
					})
				} else {
					c.JSON(http.StatusOK, &Response{
						Code:    531,
						ErrCode: fmt.Sprintf("%v", err),
						ErrMsg:  fmt.Sprintf("%v", err),
					})
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

func logPanic(c *gin.Context, err interface{}) {
	var msg string
	if be, ok := err.(*BusinessError); ok {
		msg = fmt.Sprintf("[BusinessError] %s: %s", be.ErrCode, be.ErrMsg)
	} else {
		msg = fmt.Sprintf("[Panic] %v", err)
	}

	// 获取调用栈
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)
	log.Printf("%s\nRequest: %s %s\nStack:\n%s\n", msg, c.Request.Method, c.Request.URL.Path, stack[:n])
}
