# Changelog

本文档遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范，
版本号遵循 [Semver](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.2.1] - 2026-09-19

### Changed

- **config**: `LoadEnvFile` 启动体验增强（非破坏性）
  - GO_ENV 未设置时默认按 development 处理，候选 `.env.development` → `.env`
    （本地开发零配置可跑；生产容器有进程 env 注入不受影响）
  - 候选文件从当前工作目录逐级向上查找至文件系统根，
    支持在 `cmd/` 等子目录内启动时命中项目根目录的 env 文件
  - 未命中时错误信息列出候选与起始目录

### Added

- **CI**: `ci.yml` 新增回归守卫 step —— 拒绝 `.qoder/`、`.idea/`、`.vscode/`
  下的任何文件进入索引；对非 ASCII 跟踪路径发 `::warning::`。CI 层面防止 v0.1.1 类型的
  module-zip 污染回归。不影响 module zip 内容（`.github/**` 已 `export-ignore`），
  因此不需升版。

## [0.2.0] - 2026-09-19

### Changed（破坏性）

- **config**: 配置加载改为 env-only，删除 yaml（config.yaml / config.<GO_ENV>.yaml）体系
  - `Load(dir, out)` 废除，拆为 `LoadEnvFile(explicit)`（godotenv 按 GO_ENV 加载
    `.env.development` / `.env.production`，不覆盖已存在的进程环境变量）
    与 `Load(out)`（反射遍历结构体逐 key BindEnv，env 名 = 大写路径如
    `SERVER_CORS_ORIGINS`，`[]string` 支持逗号分隔）
  - 优先级：进程环境变量 > .env.<GO_ENV> 文件
  - examples 同步：删除 `examples/conf/`，新增 `.env.development` / `.env.production`
- **config**: 新增直接依赖 `github.com/joho/godotenv`

## [0.1.1] - 2026-09-19

### Fixed

- **打包**: 将 `.qoder/` 从 git 索引中移除。该目录为 AI 助手生成的知识库镜像，
  路径含全角标点（`：`、`（`、`、`）与大写字母，会触发 Go module zip 的
  路径校验失败（`malformed file path` / `invalid name case`），导致下游项目
  `go mod tidy` / `go mod download` 直接报错。
- **工程**: 新增 `.gitattributes`，对 `.qoder/**`、`.idea/**`、`.vscode/**`、
  `.github/**` 配置 `export-ignore`，作为 archive 层面的双保险。

## [0.1.0] - 2026-09-18

初始版本。自 beego vanilla 移植全部微服务能力到 Gin + GORM。

### Added

- **vanilla**: RestResource 基类 + RestResourceInterface 契约
- **vanilla**: 约定式路由注册（资源名 → `/a/b/` + `/a/api/b/` + 别名）
- **vanilla**: 声明式参数校验（string/int/float/bool/json/json-raw/json-array，`?` 前缀可选）
- **vanilla**: 统一响应格式（MakeResponse / MakeErrorResponse）
- **vanilla**: BusinessError（panic 驱动错误流，NoPush 支持）
- **vanilla**: Filter 协议提取（`__f-字段-操作符`）
- **vanilla**: 双模式分页（offset + cursor）
- **vanilla**: 事务生命周期（非 GET 自动 Begin/Commit，panic Rollback，
  BeforeCommitCallback / AfterCommitCallback）
- **vanilla**: RepositoryBase / ServiceBase / IBusinessContextFactory
- **middleware**: Recovery / Tracing / Metrics / CORS / MethodOverride /
  JWTAuth / RequestLog / Lock 中间件 + Register 一站式挂载
- **db**: 驱动无关 Init、事务传播（ContextWithTx/TxFromContext/
  GetDBWithContext）、WithTransaction、ApplyFilters
- **cache**: go-redis 封装（Get/Set/Del/Incr/HSet/MGet/ClearByPrefix 等）
- **lock**: ILock 接口 + RedisLock(redsync) + DummyLock
- **auth**: JWT Encode/Decode/ParseUserId
- **config**: viper 加载器 + Base 配置基座
- **cron**: Task 框架（事务、Recovery、RetryPolicy、Pipe 支持）
- **client**: 服务间调用（JWT 透传 + 重试 + tracing + Bind）
- **ws**: WebSocket REST Proxy
- **tracing**: OpenTelemetry 初始化（OTLP HTTP）
- **metrics**: Prometheus 指标（endpoint/panic/business_error/restws/lru 等）
- **snowflake**: 分布式 ID（bwmarrin/snowflake 同款）
- **notify**: 钉钉机器人告警
- **lru**: 带 TTL 的内存 LRU 缓存
- **backoff**: 指数/固定退避 + Retry/RetryNotify
- **machine**: 主机信息

### Fixed（相对 beego vanilla 的缺陷修复）

- 事务完全失效：handler 内联事务管理，panic recover 触发 Rollback
- 单例 RestResource 并发 data race：每请求 reflect.New 独立实例
- 回滚判据误判：recover 路径与正常 commit 路径严格分离
