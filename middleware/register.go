package middleware

import (
	"github.com/gin-gonic/gin"
)

// Options 中间件配置选项。
// 新增字段使用零值默认行为，保证向后兼容。
type Options struct {
	// CORSOrigins CORS 允许的源列表，nil 或空表示允许所有
	CORSOrigins []string
	// JWTSecret JWT 密钥，空字符串跳过 JWT 认证
	JWTSecret string
	// EnableTx 是否启用自动事务（非 GET 请求自动 Begin/Commit）
	EnableTx bool
	// EnableLock 是否启用分布式锁
	EnableLock bool
	// EnableMetrics 是否启用 Prometheus 指标
	EnableMetrics bool
	// EnableTracing 是否启用 OpenTelemetry 链路追踪
	EnableTracing bool
	// SkipJWTPaths 跳过 JWT 认证的 URL 前缀
	SkipJWTPaths []string
	// Extra 项目自定义中间件，挂载在内置中间件之后
	Extra []gin.HandlerFunc
}

// Register 一站式注册所有内置中间件。
//
// 挂载顺序：
//  1. Recovery（panic 捕获 + tx rollback）
//  2. Tracing（span 创建）
//  3. Metrics（请求计数）
//  4. CORS（跨域）
//  5. MethodOverride（_method 参数重写）
//  6. JWT Auth（token 认证 → 注入 bContext）
//  7. Extra（项目自定义）
//  8. RequestLog（请求日志）
func Register(engine *gin.Engine, opts Options) {
	// 1. Recovery 必须最外层
	engine.Use(RecoveryMiddleware())

	// 2. Tracing
	if opts.EnableTracing {
		engine.Use(TracingMiddleware())
	}

	// 3. Metrics
	if opts.EnableMetrics {
		engine.Use(MetricsMiddleware())
	}

	// 4. CORS
	engine.Use(CORSMiddleware(opts.CORSOrigins))

	// 5. Method Override
	engine.Use(MethodOverrideMiddleware())

	// 6. JWT Auth
	if opts.JWTSecret != "" {
		engine.Use(JWTAuthMiddleware(opts.SkipJWTPaths))
	}

	// 7. 项目自定义中间件
	for _, m := range opts.Extra {
		engine.Use(m)
	}

	// 8. Request Log
	engine.Use(RequestLogMiddleware())
}
