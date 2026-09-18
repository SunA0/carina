---
kind: error_handling
name: 基于 panic/BusinessError 的统一错误处理体系
category: error_handling
scope:
    - '**'
source_files:
    - vanilla/error.go
    - vanilla/recover.go
    - vanilla/handler.go
    - vanilla/response.go
    - middleware/recovery.go
    - metrics/metrics.go
    - db/db.go
    - cache/redis.go
---

## 1. 整体方案

Carina 在 `vanilla` 包中定义了一套以 **panic + recover** 为核心的统一错误处理机制，配合统一的 HTTP 响应结构 `Response` 和 Prometheus 指标，将业务错误、系统异常与可观测性打通。

- 错误类型：`BusinessError`（`vanilla/error.go`），通过 `ErrorType` 区分 `ErrorTypeBusiness`（正常流程）与 `ErrorTypeSystem`（需要关注并推送 Sentry）。构造器包括 `NewBusinessError`、`NewSystemError`、`NewBusinessErrorFromError`，并提供 `NoPush()` / `IsNeedPush()` / `IsPanicError()` 控制是否上报监控/告警。
- 捕获与恢复：`RecoverPanic`（`vanilla/recover.go`）作为 Gin 中间件，捕获所有 panic；`middleware.RecoveryMiddleware` 仅做一层包装转发。
- 事务回滚：recover 中调用 `db.RollbackTx(c.Request.Context())`，确保 panic 时事务一定回滚；同时在 handler 的 `Prepare`/`Finish` 钩子之间用内层 recover 包裹，先回滚再重新 panic 向上冒泡。
- 指标统计：根据 panic 内容区分 `metrics.PanicCounter` 与 `metrics.BusinessErrorCounter`，分别暴露为 `carina_panic_total` 与 `carina_business_error_total`。
- 日志与堆栈：`logPanic` 打印 `[BusinessError]` 或 `[Panic]` 前缀及完整 goroutine stack。

## 2. 关键文件与职责

| 文件 | 职责 |
|---|---|
| `vanilla/error.go` | 定义 `ErrorType`、`BusinessError` 及其构造/判断方法 |
| `vanilla/recover.go` | `RecoverPanic` 中间件：回滚事务、打指标、写日志、返回统一 JSON 错误响应 |
| `vanilla/handler.go` | `CreateHandler` 反射派发，内部对事务段加 recover 保证回滚后继续上抛 panic |
| `vanilla/response.go` | 统一 `Response` 结构体与 `MakeErrorResponse` 等构造器 |
| `middleware/recovery.go` | 对外暴露 `RecoveryMiddleware()` 适配框架注册 |
| `metrics/metrics.go` | 声明 `PanicCounter`、`BusinessErrorCounter` 等指标 |
| `db/db.go` | 提供 `GetDB`/`WithTransaction`，未初始化直接 panic，并在 recover 中回滚事务 |
| `cache/redis.go` | 未初始化时 `Client()` 直接 panic，强制初始化顺序 |

## 3. 架构与约定

### 3.1 错误传播路径
1. 业务代码通过 `panic(NewBusinessError(...))` 或 `panic(NewSystemError(...))` 抛出结构化错误。
2. `CreateHandler` 在事务块内再次包裹 recover，先执行 `db.RollbackTx`，再 `panic(r)` 让上层 `RecoverPanic` 接管。
3. `RecoverPanic` 统一：
   - 回滚事务（若存在）
   - 区分 Business/System 错误并递增对应 Prometheus 计数器
   - 记录带堆栈的日志
   - 返回 HTTP 200 但 body 中 `code=500`（业务错误）或 `code=531`（未知 panic）的 `Response`
4. 调用方无需感知 panic，只需消费统一 `Response.ErrCode` / `ErrMsg`。

### 3.2 事务与锁的错误边界
- 分布式锁获取失败：`handler.go` 直接返回 `MakeErrorResponse(500, "rest:acquire_lock_failed", ...)`，不走 panic。
- 事务 Begin/Commit 失败：同样走 `MakeErrorResponse` 返回错误码。
- 事务内 panic：内层 recover 先 Rollback 再重抛，外层 recover 再 Rollback（幂等）+ 指标 + 响应。

### 3.3 基础设施初始化错误
- `db.GetDB`、`cache.Client` 在未 `Init` 时直接 `panic("... not initialized")`，属于“启动期/配置期”致命错误，由框架级 recover 兜底。
- `db.WithTransaction` 内部 recover 也遵循“先 Rollback 再 re-panic”的模式。

## 4. 约定与约束

- **业务错误必须通过 `panic(BusinessError)` 抛出**，禁止在 handler 中直接返回 error 给上层（handler 返回值被反射调用忽略）。
- **系统级异常使用 `NewSystemError`**，业务异常使用 `NewBusinessError`；两者都实现 `error` 接口，便于跨包传递后再被 recover 识别。
- **Sentry 推送策略**：默认 `needPushToSentry=true`，可通过 `NoPush()` 关闭；`ErrorTypeSystem` 始终视为需推送。
- **HTTP 状态码约定**：所有错误响应均以 `http.StatusOK` 返回，真正的状态语义由 `Response.Code` 表达（500 业务错误、531 未知 panic、405 方法不允许等）。
- **指标必填**：`metrics.Init()` 必须在应用启动时调用，否则 `PanicCounter` / `BusinessErrorCounter` 为 nil，recover 中会跳过计数（空指针保护）。
- **不可恢复的初始化错误**：`db`、`cache` 等全局资源未初始化即访问 → 直接 panic，不返回 error，强制使用者在启动阶段完成配置。
- **事务上下文隔离**：通过 `context.Context` 注入事务句柄，`GetDBWithContext` 优先取事务 DB，确保 panic 恢复时能正确定位到当前请求的事务进行回滚。