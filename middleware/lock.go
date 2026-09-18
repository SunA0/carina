package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/lock"
	"github.com/suna0/carina/vanilla"
)

// LockMiddleware 请求级分布式锁中间件。
// 适用于不走 RestResource 体系的裸路由；RestResource 路由的锁由
// vanilla/handler 按资源声明（GetLockKey/GetLockOption）自动处理。
//
// keyFunc 从请求中计算锁 key（如按用户 ID、业务单号），返回空字符串表示不加锁。
// timeout 为锁过期时间，tries 为尝试次数。
func LockMiddleware(keyFunc func(c *gin.Context) string, timeout time.Duration, tries int) gin.HandlerFunc {
	if tries <= 0 {
		tries = 1
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	pool := sync.Pool{
		New: func() interface{} {
			return &lock.LockOption{}
		},
	}

	return func(c *gin.Context) {
		key := keyFunc(c)
		if key == "" {
			c.Next()
			return
		}

		opt := pool.Get().(*lock.LockOption)
		opt.Key = key
		opt.Timeout = int(timeout / time.Second)
		opt.Tries = tries

		mutex, err := lock.Lock(key, opt)
		pool.Put(opt)

		if err != nil {
			c.AbortWithStatusJSON(http.StatusOK, vanilla.MakeErrorResponse(500,
				"middleware:acquire_lock_failed",
				fmt.Sprintf("acquire_lock_failed: %s", key)))
			return
		}
		if mutex != nil {
			defer func() {
				_, _ = mutex.Unlock()
			}()
		}

		c.Next()
	}
}
