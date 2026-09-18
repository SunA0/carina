---
kind: logging_system
name: 基于标准库 log 的简易日志输出（无结构化/分级框架）
category: logging_system
scope:
    - '**'
source_files:
    - cron/cron.go
    - middleware/request_log.go
    - vanilla/recover.go
    - notify/dingbot.go
    - ws/proxy.go
    - go.mod
---

## 1. 使用的系统/方案

本仓库 **没有引入任何第三方日志框架**（如 zap、logrus、slog），所有日志输出均使用 Go 标准库 `log` 包的 `log.Printf` / `log.Println`。依赖清单 `go.mod` 中也不存在任何日志库依赖。

因此，该仓库不存在统一的日志初始化、全局 logger 实例、日志级别管理、结构化字段或自定义 sink —— 每个包各自直接调用 `log.Printf` 输出字符串化日志。

## 2. 关键文件与位置

- `cron/cron.go`：定时任务生命周期日志，统一在 `taskWrapper` 中通过 `log.Printf("[cron] ...")` 记录 begin tx、panic、run、failed/done、create task、started tasks、stop tasks 等事件。
- `middleware/request_log.go`：Gin 请求日志中间件，以 `[carina]` 前缀输出 `method path statusCode latency` 一行式访问日志。
- `vanilla/recover.go`：Panic 恢复中间件，在 `logPanic` 中拼接 `[BusinessError]` / `[Panic]` 前缀并附带 HTTP 方法、URL 路径和 `runtime.Stack` 堆栈信息后通过 `log.Printf` 输出。
- `notify/dingbot.go`：钉钉告警通知，当 `Token` 为空或 `Mode == "develop"` 时降级为仅打印 `[dingbot]` 前缀日志而不发送网络请求。
- `ws/proxy.go`：WebSocket 代理，在 upgrade 失败等异常路径下通过 `log.Printf("[ws] upgrade error: %v", err)` 输出。

## 3. 架构与约定

- **无集中式 Logger**：各包独立 import `log` 并直接调用，不存在全局 logger 配置入口，也没有对外暴露的日志 API。
- **前缀约定**：日志行普遍采用方括号模块名前缀作为第一字段，便于快速区分来源：
  - `[cron]`：定时任务相关
  - `[carina]`：HTTP 请求访问日志
  - `[dingbot]`：钉钉通知相关
  - `[ws]`：WebSocket 相关
  - Panic 场景使用 `[BusinessError]` / `[Panic]` 前缀
- **格式约定**：全部为人类可读的字符串拼接，未使用 JSON 或其他结构化格式；错误信息通常放在 `%v` 占位符中。
- **无日志级别**：代码中没有 INFO/WARN/ERROR/FATAL 等分级概念，所有输出都是同等严重程度的 `log.Printf`。
- **与可观测性解耦**：日志输出与 metrics（Prometheus 计数器/仪表盘）和 tracing（OpenTelemetry span）是并列的独立通道——metrics 通过计数器递增，tracing 通过 `tracing.StartSpan` 创建 span，而日志则走标准库 `log`。

## 4. 约定与约束

- **约束：禁止引入第三方日志库**——当前所有包都只依赖标准库 `log`，这是由现有实现事实所体现的约束；若未来要替换日志后端，需逐个包修改调用点。
- **约定：新增日志应遵循 `[module] message` 前缀风格**，以便在终端或日志聚合系统中按模块过滤。
- **约定：Panic 恢复路径必须同时输出堆栈**——`vanilla/recover.go` 中的 `logPanic` 强制附加 `runtime.Stack` 输出，作为 panic 调试的最低要求。
- **约定：开发环境降级行为**——`notify/dingbot.go` 在 `Mode == "develop"` 或无 token 时仅打印日志不发起网络请求，这是一种通过日志代替实际副作用的降级策略。
- **约束：无结构化字段**——日志不包含 key-value 字段，无法被外部结构化日志解析器直接消费；如需结构化日志，需在应用层自行封装。
- **约束：无日志级别控制**——无法通过配置开关不同级别的日志输出，所有 `log.Printf` 都会写入默认 stderr。

总结：这是一个极简的日志体系，完全基于 Go 标准库 `log`，通过统一的前缀约定来组织输出，没有提供框架级的日志能力；可观测性通过 metrics 和 tracing 两个独立子系统实现。