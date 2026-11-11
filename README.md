# Carina

基于 Gin + GORM 的微服务基础框架，移植 vanilla（beego）的全部微服务能力。

## 特性

- **RestResource 体系** — 声明式参数校验、约定式路由、统一响应格式
- **事务生命周期** — 非 GET 请求自动 Begin/Commit，panic 自动 Rollback
- **分布式锁** — 资源级锁声明（redsync），支持裸路由锁中间件
- **Filter 协议** — `__f-字段-操作符` 查询参数自动转 GORM Where
- **双模式分页** — offset 分页（后台）+ cursor 分页（API 服务）
- **JWT 认证** — token 编解码、业务上下文注入
- **服务间调用** — 带 JWT + 重试 + tracing 的 HTTP 客户端
- **Cron 任务框架** — 事务、Recovery、重试策略一体化
- **WebSocket REST Proxy** — 通过 WS 通道派发内部 REST 请求
- **可观测性** — Prometheus metrics + OpenTelemetry tracing
- **工具集** — snowflake / LRU / backoff / 钉钉告警

## 安装

```bash
go get github.com/suna0/carina@latest
```

要求 Go 1.21+。框架**不引入任何 DB 驱动**（mysql/postgres 由项目自行 require）。

## 快速开始

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/suna0/carina/cache"
    "github.com/suna0/carina/config"
    "github.com/suna0/carina/db"
    "github.com/suna0/carina/middleware"

    _ "github.com/go-sql-driver/mysql"
    "gorm.io/driver/mysql"
)

func main() {
    // 1. 加载配置（项目本地 config 内嵌 config.Base）
    //    优先级：进程环境变量 > .env.<GO_ENV> 文件
    config.LoadEnvFile("")
    cfg := &AppConfig{}
    config.Load(cfg)

    // 2. 初始化基础设施
    db.Init(mysql.Open(cfg.DB.DSN), &db.Options{
        MaxIdleConns: cfg.DB.MaxIdle,
        MaxOpenConns: cfg.DB.MaxOpen,
    })
    cache.Init(cache.Options{
        Address:  cfg.Redis.Address,
        Password: cfg.Redis.Password,
        DB:       cfg.Redis.DB,
    })

    // 3. 创建引擎 + 注册中间件
    engine := gin.New()
    middleware.Register(engine, middleware.Options{
        CORSOrigins: cfg.Server.CORSOrigins,
        JWTSecret:   cfg.Auth.JWTSecret,
        EnableTx:    true,
        EnableLock:  true,
    })

    // 4. 注册资源路由
    rest.RegisterAll(engine)

    // 5. 启动
    engine.Run(cfg.Server.Host + ":" + fmt.Sprint(cfg.Server.Port))
}
```

## 编写 Resource

```go
package account

import (
    "github.com/suna0/carina/vanilla"
)

type Corp struct {
    vanilla.RestResource
}

func (r *Corp) Resource() string { return "account.corp" }

func (r *Corp) GetParameters() map[string][]string {
    return map[string][]string{
        "GET":  {"?id:int", "?uuid:string"},
        "POST": {"name:string", "data:json"},
    }
}

func (r *Corp) Get() {
    bCtx := r.GetBusinessContext()
    corpId, _ := r.GetInt("id")

    repo := account.NewCorpRepository(bCtx)
    corp := repo.GetById(corpId)
    if corp == nil {
        panic(vanilla.NewBusinessError("corp:not_found", "企业不存在"))
    }

    r.ReturnJSON(vanilla.MakeResponse(vanilla.Map{
        "id": corp.Id, "name": corp.Name,
    }))
}
```

路由注册后自动获得：

- `GET/POST/PUT/DELETE /account/corp/`
- `ANY /account/api/corp/`
- 非 GET 请求自动包裹事务
- `GetLockKey()` 非空时自动加分布式锁

## 事务提交后输出响应

新增能力通过 `HandlerOptions` 显式启用，适用于普通、非流式的 REST 响应：

```go
vanilla.RouterWithOptions(engine, &account.Corp{}, vanilla.HandlerOptions{
    BufferResponseUntilCommit: true,
    MaxResponseBytes:          1 << 20, // 可选；默认正文上限为 1 MiB
})
```

自行挂载 handler 时使用 `vanilla.CreateHandlerWithOptions(resource, opts)`。
资源中的 `ReturnJSON()`、`ReturnJSONWithCallback()` 和直接 `Ctx.JSON()` 调用无需修改。
标准路由、API 路由和别名使用相同选项。

启用后，开启自动事务的请求先将状态码、响应头和已序列化正文暂存在当前请求中，
提交成功后才输出。提交失败时丢弃暂存内容，只返回一份
`code=500, errCode=rest:tx_commit_failed` 响应，HTTP 状态仍为 200。
提交前 panic 会回滚并由 Recovery 输出错误；渲染失败、缓冲超限或 handler 执行期间
新增的 `c.Error()` 会回滚并返回 `rest:response_failed`。
`ReturnError()` 和响应体中的业务码本身不会自动触发回滚。

响应在提交成功时冻结。随后按顺序执行 `AfterCommitCallback` 和 `Finish()`，最后输出响应；
这两个钩子中的 panic 会记录到日志和 Gin 错误列表，不回滚已提交事务，也不替换已确定的响应。
提交后钩子不能再写响应或修改其响应头，需要影响响应或事务成败的工作应在提交前完成。
网络写入失败仅记录错误，不会再追加一份 JSON。提交失败不代表能确认数据库一定未提交，
框架也不会自动重试请求或提交后回调。

该选项不设置事务、Commit 或提交后钩子的超时时间。请求已有的 Context deadline 会传递
给事务，但正在进行的 Commit 能否及时中断仍取决于驱动及网络超时配置；
提交后钩子阻塞也会延迟响应。下游应结合实际驱动和数据库设置等待上限。

缓冲超限不会降级为提前输出。启用自动事务缓冲时不支持 `Flush()` / SSE / `Hijack()`，
这类资源应通过 `DisableTx()` 关闭自动事务，并自行限定数据库事务范围。
上游中间件已经写出响应时，handler 会终止请求而不执行事务业务。
该选项不缓冲进入 handler 之前或 handler 返回之后的中间件写入。

**旧入口 `Router()`、`CreateHandler()` 和零值 Options 保持即时输出行为；仅升级依赖不会启用修复。**
`DisableTx()` 返回 true 的请求也保持即时输出。该能力按 minor 版本发布，
未修改 `RestResourceInterface` 等关键接口。

## Filter 查询协议

```
GET /account/corp/?__f-name-contain=张&__f-age-gt=18&__f-status-in=1,2
```

在业务层：

```go
db := vanilla.GetDBFromContext(ctx)
db = carinadb.ApplyFilters(db, r.GetFilters())
// → WHERE name LIKE '%张%' AND age > 18 AND status IN (1,2)
```

支持操作符：`equal / contain / gt / gte / lt / lte / in / notin / range`

## 版本策略

遵循 Semver：

| 变更 | 版本 | 下游动作 |
|---|---|---|
| Bug fix | patch | `go get -u` 直接升 |
| 新增能力（向后兼容） | minor | 按需升级 |
| Breaking change | major | 按迁移指南改代码 |

任何 public API 从 Deprecated 到删除至少跨 **2 个 minor 版本**。

## 文档

- [ARCHITECTURE.md](ARCHITECTURE.md) — 架构设计与模块职责
- [CHANGELOG.md](CHANGELOG.md) — 版本变更记录
- [AGENTS.md](AGENTS.md) — 协作约束与开发规范
