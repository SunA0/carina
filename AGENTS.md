# AGENTS.md — Carina 框架仓协作约束

本文件约束 AI 协作者与人类开发者在**框架仓**内的行为。

## 仓定位

Carina 是所有后端项目的基础框架（Gin + GORM）。
下游项目通过 `go get github.com/suna0/carina@vX.Y.Z` 依赖本仓。

## 硬约束

1. **禁止引入 DB 驱动** — 不得 import `go-sql-driver/mysql`、`pgx` 等
   具体驱动，驱动由下游项目选择。
2. **禁止引入业务代码** — 不得 import 任何下游项目包。
3. **public API 变更走 Semver** —
   - 新增能力：minor 版本，用 Options 零值字段保持兼容
   - 废弃 API：标记 `Deprecated:` 注释，至少跨 2 个 minor 才删除
   - Breaking change：major 版本 + 迁移指南 + codemod 脚本
4. **关键接口不得静默修改** —
   `vanilla.RestResourceInterface` / `vanilla.IBusinessContextFactory` /
   `lock.ILock` / `cron.TaskInterface` 的变更必须 major 版本。
5. **依赖方向单向** — 见 ARCHITECTURE.md「依赖方向」，禁止反向依赖。
6. **全局状态初始化幂等** — metrics.Init / db.Init 等使用 sync.Once，
   重复调用不得 panic 或重复注册。

## 代码规范

- 所有导出符号必须有中文 doc 注释
- 错误信息前缀包名，如 `db: begin tx: %w`
- 并发访问共享状态必须有锁或原子操作保护
- 每请求的状态不得挂到全局/单例对象上（vanilla 的历史教训）
- `go build ./...` 与 `go vet ./...` 必须通过

## 变更清单（提交前自查）

- [ ] CHANGELOG.md 已更新（Added/Changed/Deprecated/Removed/Fixed 分类）
- [ ] 新增 public API 已评估兼容性
- [ ] 无新增第三方重型依赖（优先标准库）
- [ ] `go mod tidy` 已执行

## 测试

- 核心机制（事务传播、Filter 转换、参数校验、分页）应有单元测试
- 测试不得依赖真实 DB / Redis（使用 mock 或接口注入）
