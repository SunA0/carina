# Carina 架构说明

## 设计目标

把 vanilla（beego fork）验证过的微服务能力完整移植到 Gin + GORM，
同时修复 vanilla 的三个已知缺陷：

1. **事务完全失效** → handler 内联事务管理，panic 触发 rollback
2. **单例 RestResource 并发 data race** → 每请求 `reflect.New` 独立实例
3. **回滚判据误判** → panic recover 与 commit 路径严格分离

## 模块职责

```
carina/
├── vanilla/      核心契约层：RestResource 体系、路由、参数校验、响应、
│                 错误、分页、Filter 提取、BusinessContext 契约
├── middleware/   Gin 中间件集 + Register 一站式挂载
├── db/           GORM 封装：驱动无关 Init、事务传播、ApplyFilters
├── cache/        go-redis 封装：常用操作 + 前缀清理
├── lock/         分布式锁：ILock 接口 + RedisLock(redsync) + DummyLock
├── auth/         JWT 编解码（Claims: userId/authUserId/tokenType）
├── config/       env-only 加载器（godotenv + viper BindEnv）+ Base 通用配置基座
├── cron/         robfig/cron 封装：事务、Recovery、重试策略
├── client/       服务间调用：JWT 透传 + 重试 + tracing
├── ws/           WebSocket REST Proxy：WS 通道派发内部 REST 请求
├── tracing/      OpenTelemetry 初始化（OTLP HTTP exporter）
├── metrics/      Prometheus 指标定义（Init 幂等注册）
├── snowflake/    分布式 ID 生成（bwmarrin/snowflake 同款）
├── notify/       钉钉机器人告警
├── lru/          带 TTL 的内存 LRU 缓存
├── backoff/      重试退避算法（指数/固定/零/停止）
└── machine/      主机信息（hostname/ip → response _pod 字段）
```

## 依赖方向（单向，禁反向）

```
middleware → vanilla → {db, lock, metrics, auth}
cron       → {db, client, tracing, metrics}
ws         → {vanilla（经 engine 内部派发）, metrics}
client     → {auth, tracing, metrics, backoff}
其余包（config/cache/lock/auth/tracing/metrics/snowflake/notify/lru/backoff/machine）
    均为叶子包，不依赖 carina 内部其他包
```

## 请求生命周期

```
Request
  ├─ middleware.Recovery      defer 捕获 panic → 错误响应
  ├─ middleware.Tracing       创建 span（可选）
  ├─ middleware.Metrics       请求计数（可选）
  ├─ middleware.CORS          跨域处理
  ├─ middleware.MethodOverride _method 参数重写
  ├─ middleware.JWTAuth       解码 token → 注入 bContext（可选）
  │
  ├─ vanilla.CreateHandler
  │    ├─ 每请求 reflect.New 独立实例（并发安全）
  │    ├─ parseParameters     按 GetParameters() 声明校验
  │    ├─ 分布式锁            GetLockKey() 非空时获取
  │    ├─ 事务                DisableTx()=false 时 Begin
  │    │    └─ defer recover → Rollback
  │    ├─ Prepare()
  │    ├─ 反射派发 Get/Post/Put/Delete
  │    ├─ BeforeCommitCallback
  │    ├─ Commit
  │    ├─ AfterCommitCallback
  │    └─ Finish()
  │
  └─ Response 返回
```

## 事务传播

```go
// 框架侧：handler 开启事务并注入 request context
txCtx, tx, _ := db.BeginTx(c.Request.Context())
c.Request = c.Request.WithContext(txCtx)

// 业务侧：RepositoryBase.DB() 自动感知事务
func (r *RepositoryBase) DB() *gorm.DB {
    return db.GetDBWithContext(r.Ctx) // 有 tx 用 tx，否则用全局 DB
}
```

裸路由/独立场景使用 `db.WithTransaction(ctx, fn)`。

## 并发安全

- vanilla 原版：单例 RestResource，每请求写入共享字段 → data race
- Carina：每请求 `reflect.New(resourceType)` 创建独立实例，
  元数据（method 句柄、参数声明）启动时预计算缓存

## 契约边界

**进框架（机制）**：RestResource 基类、路由注册、参数校验、响应格式、
事务生命周期、分布式锁、JWT 流程、配置加载、Redis 客户端、Cron 运行时、
Tracing/Metrics 初始化、Panic Recovery、服务间调用、WS Proxy、
Filter 转换、分页。

**留项目（策略/业务）**：具体 Resource 实现、BusinessContextFactory、
数据模型、业务逻辑、Cron Task 实现、配置业务字段、DB 驱动选择、
项目自定义中间件。

## 关键接口（稳定性契约）

以下接口一旦发布，breaking change 需 major 版本：

- `vanilla.RestResourceInterface`
- `vanilla.IBusinessContextFactory`
- `lock.ILock`
- `cron.TaskInterface`

## 兼容性保障

1. **Options 模式** — 新能力通过零值字段 opt-in，不改已有行为
2. **Semver + Go Module** — 下游锁版本，自主决定升级时机
3. **Deprecation 生命周期** — 废弃到删除至少跨 2 个 minor
4. **go.work 联动** — 本地开发即时验证框架改动
5. **兼容性 CI** — 框架 PR 自动对下游跑 build/test
