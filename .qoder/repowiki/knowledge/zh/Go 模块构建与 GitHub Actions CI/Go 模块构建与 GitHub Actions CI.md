---
kind: build_system
name: Go 模块构建与 GitHub Actions CI
category: build_system
scope:
    - '**'
source_files:
    - .github/workflows/ci.yml
    - .github/workflows/compat.yml
    - go.mod
---

## 1. 构建系统概览

该项目是一个 Go 模块（`module github.com/suna0/carina`），使用标准 `go build` / `go test` 工具链进行编译与测试，没有自定义 Makefile、Dockerfile 或 shell 构建脚本。所有构建与质量检查流程通过 GitHub Actions 在 `.github/workflows/` 中定义。

- **Go 版本**：`go.mod` 声明 `go 1.22`；CI 使用 `actions/setup-go@v5` 安装 `go-version: "1.21"` 执行构建与测试（两者存在版本差异，实际运行环境为 1.21）。
- **依赖管理**：完全基于 `go.mod` / `go.sum`，无 vendor 目录，无第三方包管理器。
- **产物**：仅产生可执行二进制（由下游服务 `go build ./...` 产出），本仓库本身不发布独立镜像或归档。

## 2. 关键文件

- `.github/workflows/ci.yml`：主 CI 流水线，触发条件为 push 到 `main` / `master` 分支以及所有 pull_request。
- `.github/workflows/compat.yml`：兼容性 CI，用于框架发版前对下游项目执行替换编译检查（当前因 `if: ${{ false }}` 默认跳过，需登记下游项目后启用）。
- `go.mod`：模块声明、Go 版本约束及全部直接/间接依赖。

## 3. 架构与约定

### 3.1 主 CI（ci.yml）
流水线按顺序执行四步：
1. `go build ./...` — 全量编译校验。
2. `go vet ./...` — 静态分析。
3. `go test -race -coverprofile=coverage.out ./...` — 并发安全检测 + 覆盖率收集。
4. `staticcheck ./...` — 通过 `honnef.co/go/tools/cmd/staticcheck@2023.1.7` 做更严格的 lint（以 `|| true` 降级为非阻塞）。

### 3.2 兼容性 CI（compat.yml）
设计意图是“框架发版前自动对所有下游项目跑编译检查”。其工作方式为：
- 同时 checkout 本仓库（路径 `carina`）和下游仓库（路径 `downstream`）。
- 在下游仓库中执行 `go mod edit -replace github.com/suna0/carina=../carina` 将依赖指向 PR 分支的本地副本。
- 随后 `go mod tidy` 并 `go build ./...` / `go test ./...`。
- 矩阵 `matrix.project` 预留了 `{ repo, ref }` 格式，但当前为空且整体被 `if: ${{ false }}` 禁用，待下游就绪后移除该限制即可启用。

### 3.3 版本与发布
- 未发现任何版本号字段、changelog 自动生成脚本或发布标签规则。
- 版本管理由外部流程控制（CHANGELOG.md 存在，但未集成到 CI 中）。
- 由于没有 Dockerfile / Makefile / release 脚本，本项目作为 Go 库被下游通过 `go get` 引用，不存在独立的制品发布步骤。

## 4. 约定与约束

- **构建命令**：统一使用 `go build ./...` 覆盖整个模块，禁止只编译单个包。
- **测试命令**：必须包含 `-race` 标志，强制开启竞态检测；覆盖率输出到 `coverage.out`。
- **Lint 策略**：`go vet` 为硬性检查；`staticcheck` 作为可选增强（失败不阻断 CI）。
- **Go 版本锁定**：CI 固定 `go-version: "1.21"`，与 `go.mod` 中的 `go 1.22` 不一致，实际受限于 runner 安装的版本。
- **缓存**：启用 `actions/setup-go` 的内置缓存加速依赖下载。
- **兼容性保障**：通过 compat workflow 的 `-replace` 机制确保框架变更不会破坏下游编译，但该功能目前处于禁用状态。
- **分支策略**：CI 仅在 `main` / `master` 分支 push 时触发，PR 也会触发完整流水线。

## 5. 缺失项

- 无 Makefile / build.sh / Dockerfile / docker-compose.yml。
- 无交叉编译目标（未指定 GOOS/GOARCH）。
- 无制品上传（如 `actions/upload-artifact`）。
- 无语义化版本发布流程（无 tag 推送触发 release）。