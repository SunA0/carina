---
kind: dependency_management
name: Go 模块依赖管理（go.mod + go.sum，无 vendor）
category: dependency_management
scope:
    - '**'
source_files:
    - go.mod
    - go.sum
    - .github/workflows/ci.yml
---

## 1. 使用的系统/方案

- **Go Modules**：项目使用 Go 官方模块系统进行第三方依赖声明与管理，根目录包含 `go.mod` 与 `go.sum`。
- **Go 版本**：`go.mod` 声明 `go 1.22`；CI 中 `actions/setup-go@v5` 安装的是 `go-version: "1.21"`，存在 CI 与模块声明不一致的情况。
- **无 vendoring**：仓库未提交 `vendor/` 目录，也未见 `.gitignore` 中排除 `vendor/` 的条目，依赖通过 Go Module Cache 在构建时拉取。
- **无私有代理/替换**：`go.mod` 中未发现 `replace` 指令、`GOPRIVATE` 或自定义 GOPROXY 配置，所有依赖均从公共 Go Proxy / GitHub 拉取。

## 2. 关键文件

- `go.mod`：定义 module `github.com/suna0/carina`、Go 版本以及全部直接依赖与间接依赖。
- `go.sum`：锁定每个依赖的精确版本哈希（由 `go mod tidy` 生成）。
- `.github/workflows/ci.yml`：CI 流水线执行 `go build ./...`、`go vet ./...`、`go test -race -coverprofile=coverage.out ./...` 以及 `staticcheck ./...`，依赖来源完全由 Go Modules 解析。

## 3. 架构与约定

- **直接依赖集中声明**：所有被代码直接 import 的库集中在 `require` 块中，包括 Gin (`gin-gonic/gin`)、GORM (`gorm.io/gorm`)、Redis 客户端 (`redis/go-redis/v9`)、分布式锁 (`go-redsync/redsync/v4`)、JWT (`golang-jwt/jwt/v5`)、Cron (`robfig/cron/v3`)、Viper (`spf13/viper`)、Prometheus (`prometheus/client_golang`)、OpenTelemetry (`otel/*`)、WebSocket (`gorilla/websocket`) 等。
- **间接依赖自动管理**：大量第三方库以 `// indirect` 标记出现在 `go.mod` 中，由 Go 工具链根据直接依赖推导并锁定。
- **单向包结构**：各子包（如 `auth`、`db`、`cron`、`middleware` 等）仅向上层暴露接口，不反向引用其他业务包，使依赖关系清晰、便于独立升级单个第三方库。
- **无本地替换**：未发现 `replace` 指向 fork 或私有仓库，说明框架对上游依赖保持“即插即用”风格。

## 4. 约定与约束

- **版本锁定**：所有依赖（含间接依赖）均由 `go.sum` 锁定精确版本，保证构建可重复。
- **依赖更新方式**：应通过 `go get -u ./...` 或 `go mod tidy` 更新，并由 CI 校验 `go build`、`go vet`、`go test -race` 是否通过。
- **CI 强制检查**：每次 push/PR 都会运行 `go build`、`go vet`、`go test -race` 以及 `staticcheck`，任何破坏依赖兼容性或引入问题的变更会被拦截。
- **Go 版本一致性待修正**：`go.mod` 声明 `go 1.22`，而 CI 使用 `go 1.21`，建议统一以避免因语言特性差异导致的构建/行为不一致。
- **无私有源策略**：当前仓库未配置私有 Go 代理或 `GOPRIVATE`，若未来引入企业内部库，需补充相应配置。
- **无 vendor 策略**：依赖不在仓库内快照化，构建依赖网络可达的 Go 模块代理；如需离线构建或严格锁定，需额外引入 `go mod vendor` 流程。