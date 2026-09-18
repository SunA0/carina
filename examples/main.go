// Carina 框架使用示例（完整启动流程）。
//
// 运行：
//
//	cd examples && GO_ENV=dev go run .     # 加载 .env.development
//	GO_ENV=prod go run .                   # 加载 .env.production
//
// 配置来源：进程环境变量 > .env.<GO_ENV> 文件（env-only，不再读取 yaml）。
// 无 DB/Redis 环境也能跑：DSN / Redis 地址为空时自动跳过对应初始化。
package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/auth"
	"github.com/suna0/carina/cache"
	carinaconfig "github.com/suna0/carina/config"
	"github.com/suna0/carina/db"
	"github.com/suna0/carina/lock"
	"github.com/suna0/carina/metrics"
	"github.com/suna0/carina/middleware"
	"github.com/suna0/carina/tracing"
	"github.com/suna0/carina/vanilla"

	"gorm.io/driver/mysql"

	"example/config"
	"example/rest/demo"
)

func main() {
	// 1. 加载配置：进程环境变量 > .env.<GO_ENV> 文件
	if file, err := carinaconfig.LoadEnvFile(""); err != nil {
		log.Printf("[example] %v, continue with process env vars only", err)
	} else {
		log.Printf("[example] loaded env file %s", file)
	}
	cfg := &config.Config{}
	if err := carinaconfig.Load(cfg); err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("[example] app=%s greeting=%s env_mode=%s",
		cfg.AppName, cfg.Greeting, cfg.Server.Mode)

	// 2. 初始化基础设施（按配置条件初始化，缺啥跳啥）
	initInfra(cfg)

	// 3. Gin 引擎 + 框架中间件一站式注册
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	middleware.Register(engine, middleware.Options{
		CORSOrigins:   cfg.Server.CORSOrigins,
		JWTSecret:     cfg.Auth.JWTSecret,
		EnableMetrics: true,
		EnableTracing: cfg.Tracing.Enabled,
	})

	// 4. 注册资源路由（约定路由：/demo/user/ + /demo/api/user/）
	// 原型实例的自定义字段不会传递给每请求新实例，配置用包级变量注入
	demo.Greeting = cfg.Greeting
	vanilla.Router(engine, &demo.User{})

	// 5. 健康检查裸路由（演示框架外的普通 Gin 用法可自由混用）
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, vanilla.MakeResponse(vanilla.Map{"status": "ok"}))
	})

	// 6. 启动
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("[example] listening on %s", addr)
	if err := engine.Run(addr); err != nil {
		log.Fatalf("run: %v", err)
	}
}

// initInfra 按配置条件初始化 DB / Redis / 锁 / 指标 / 追踪
func initInfra(cfg *config.Config) {
	// 指标（幂等，任何地方调用一次即可）
	metrics.Init()

	// JWT 密钥
	if cfg.Auth.JWTSecret != "" {
		auth.Init(cfg.Auth.JWTSecret)
	}

	// 数据库：DSN 由 DB_DSN 环境变量注入，为空则跳过（示例可裸跑）
	if cfg.DB.DSN != "" {
		if err := db.Init(mysql.Open(cfg.DB.DSN), &db.Options{
			MaxIdleConns: cfg.DB.MaxIdle,
			MaxOpenConns: cfg.DB.MaxOpen,
		}); err != nil {
			log.Fatalf("init db: %v", err)
		}
		log.Printf("[example] db initialized")
	} else {
		log.Printf("[example] DB_DSN not set, skip db.Init (GET 请求不受影响)")
	}

	// Redis + 分布式锁
	if cfg.Redis.Address != "" {
		if err := cache.Init(cache.Options{
			Address:  cfg.Redis.Address,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		}); err != nil {
			log.Fatalf("init redis: %v", err)
		}
		if cfg.Lock.Engine == "redis" {
			lock.InitRedis(cache.Client())
			log.Printf("[example] redis lock engine initialized")
		}
	} else {
		log.Printf("[example] REDIS_ADDRESS not set, skip cache/lock init")
	}

	// 链路追踪
	if cfg.Tracing.Enabled {
		shutdown, err := tracing.Init(cfg.AppName, tracing.Options{
			Enabled:    cfg.Tracing.Enabled,
			Endpoint:   cfg.Tracing.Endpoint,
			SampleRate: cfg.Tracing.SampleRate,
		})
		if err != nil {
			log.Printf("[example] init tracing failed: %v", err)
		} else {
			_ = shutdown // 生产环境应在退出前调用 shutdown(context.Background())
		}
	}
}
