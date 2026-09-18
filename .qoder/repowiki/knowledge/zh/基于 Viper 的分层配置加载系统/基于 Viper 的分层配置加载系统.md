---
kind: configuration_system
name: 基于 Viper 的分层配置加载系统
category: configuration_system
scope:
    - '**'
source_files:
    - config/config.go
    - config/types.go
---

## 1. 使用的系统与框架

该仓库使用 `github.com/spf13/viper` 作为统一的配置加载与合并引擎，封装在 `config` 包中。通过 Viper 实现 YAML 配置文件读取、环境变量覆盖、多环境文件合并以及结构体反序列化。

## 2. 核心文件

- `config/config.go`：配置加载入口 `Load(dir string, out any) error`，集中处理 Viper 初始化、环境变量绑定、YAML 文件读取与环境覆盖。
- `config/types.go`：定义框架通用配置结构体，包括 `ServerConfig`、`DBConfig`、`RedisConfig`、`AuthConfig`、`TracingConfig`、`LockConfig` 以及组合基座 `Base`。

## 3. 架构与约定

### 加载顺序（严格遵循）

1. **启用环境变量优先**：调用 `v.AutomaticEnv()` 并设置 `SetEnvKeyReplacer(".", "_")`，使形如 `db.dsn` 的配置键可通过环境变量 `DB_DSN` 覆盖。
2. **敏感字段显式绑定**：对 `db.dsn`、`redis.address`、`redis.password`、`auth.jwt_secret` 四个字段调用 `BindEnv`，确保这些敏感信息只能通过环境变量注入，不落地到配置文件。
3. **基础配置文件**：从指定目录 `dir` 读取 `config.yaml`（必需），若不存在直接报错。
4. **环境覆盖文件**：读取 `GO_ENV`（优先从 Viper 的 `GO_ENV` 键获取，否则回退到 `os.Getenv("GO_ENV")`），若非空则尝试合并 `config.<env>.yaml`；若该环境文件不存在，仅打印提示而不报错，允许只使用基础配置运行。
5. **反序列化**：最终通过 `Unmarshal(out)` 将完整配置映射到调用方传入的结构体。

### 配置结构约定

`types.go` 提供了一套框架内置的配置模型，以 `Base` 为根节点组织各子系统配置：

| 顶层键 | 结构体 | 用途 |
|---|---|---|
| `server` | `ServerConfig` | HTTP 服务监听地址、端口、模式、CORS 白名单 |
| `db` | `DBConfig` | GORM DSN、连接池大小 |
| `redis` | `RedisConfig` | Redis 地址、密码、库号 |
| `auth` | `AuthConfig` | JWT 密钥 |
| `tracing` | `TracingConfig` | 是否启用、OTLP endpoint、采样率 |
| `lock` | `LockConfig` | 锁引擎选择（dummy/redis）及 Redis 参数 |

业务方应在本地 `config` 包内嵌 `Base` 并追加自身业务字段，从而复用框架的加载逻辑。

### 环境变量命名规则

由于设置了 `SetEnvKeyReplacer(".", "_")`，所有配置项都支持以下两种访问方式：
- 配置文件中的点号路径：`db.dsn`
- 环境变量中的下划线路径：`DB_DSN`

此外，`GO_ENV` 本身也同时支持通过 Viper 键和 `os.Getenv` 两种方式读取，用于决定加载哪个环境覆盖文件。

## 4. 约定与约束

- **配置文件必须存在**：`config.yaml` 是必需的，缺失时 `Load` 直接返回错误。
- **环境覆盖文件可选**：`config.<env>.yaml` 不存在不会导致启动失败，仅打印一行提示日志。
- **敏感字段强制走环境变量**：`db.dsn`、`redis.address`、`redis.password`、`auth.jwt_secret` 通过 `BindEnv` 绑定，设计上要求这些值由环境变量注入，避免写入配置文件。
- **多环境通过 `GO_ENV` 切换**：约定使用 `GO_ENV=prod|dev|test` 等值来选择对应的 `config.<env>.yaml` 覆盖文件。
- **统一入口**：所有配置加载都通过 `config.Load(dir, &out)` 完成，禁止各模块自行实例化 Viper，保证加载顺序一致。
- **配置结构可组合**：业务项目应内嵌 `Base` 而非复制其字段，以便随框架演进自动获得新配置项。