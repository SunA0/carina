package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/metrics"
)

// MetricsMiddleware Prometheus 指标收集中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		c.Next()

		if metrics.EndpointCounter != nil {
			metrics.EndpointCounter.WithLabelValues(path, method).Inc()
		}
		if metrics.EndpointDuration != nil {
			metrics.EndpointDuration.WithLabelValues(path, method).Observe(time.Since(start).Seconds())
		}
	}
}
