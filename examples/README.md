# Carina 使用示例

演示下游项目基于 Carina 框架的标准用法：配置加载、基础设施初始化、
中间件注册、约定式资源路由、声明式参数校验、统一响应、错误流。

## 目录结构

```
examples/
├── main.go            # 启动入口（完整流程）
├── .env.development   # GO_ENV=dev 时加载的配置（env-only，无 yaml）
├── .env.production    # GO_ENV=prod 时加载的配置
├── config/
│   └── config.go      # 项目配置：内嵌 carina config.Base + 业务字段
└── rest/
    └── demo/
        └── user.go    # 示例资源（GET/POST/PUT）
```

## 运行

```bash
cd examples
go mod tidy
GO_ENV=dev go run .
```

无需 DB / Redis 即可启动（相关初始化自动跳过）。

## 验证

```bash
# GET：可选参数 + 统一响应
curl "http://127.0.0.1:8080/demo/user/?id=42&name=zhangsan"

# Filter 协议：__f-字段-操作符（curl 需 -g 关闭 URL globbing）
curl -g "http://127.0.0.1:8080/demo/user/?__f-age-gt=18&__f-id-in=[1,2,3]"

# POST：声明式参数校验（缺 name 会返回参数错误响应）
curl -X POST "http://127.0.0.1:8080/demo/user/" \
     -d 'name=lisi' \
     -d 'profile={"age":20}' \
     -d 'tags=["a","b"]'

# BusinessError panic → 统一错误响应（code 500 + errCode）
curl -X POST "http://127.0.0.1:8080/demo/user/" -d 'name=admin'

# 未实现的方法 → 405
curl -X DELETE "http://127.0.0.1:8080/demo/user/"

# API 别名路由
curl "http://127.0.0.1:8080/demo/api/user/?id=1"

# 裸路由混用
curl "http://127.0.0.1:8080/healthz"
```

## 环境变量与多环境

配置来源：进程环境变量 > .env.<GO_ENV> 文件（dev/development → .env.development，
prod/production → .env.production，其他值 → .env.<GO_ENV>，无 GO_ENV 时回退 .env）。

环境变量名 = 配置路径大写且 `.` 换 `_`，如 `server.cors_origins` → `SERVER_CORS_ORIGINS`
（列表用逗号分隔）。

```bash
# 敏感字段注入（不写入 env 文件，进程环境变量优先级最高）
export DB_DSN="user:pass@tcp(127.0.0.1:3306)/demo?charset=utf8mb4&parseTime=True&loc=Local"
export REDIS_ADDRESS="127.0.0.1:6379"
export AUTH_JWT_SECRET="your-secret"

# 多环境：加载 .env.production（端口变 9090、release 模式）
GO_ENV=prod go run .
```

## 关键约定

- **资源实例**：框架每请求创建资源新实例（并发隔离），原型上的自定义字段
  不会传递。项目侧配置请通过包级变量注入（见 rest/demo/user.go 的 Greeting）。
- **错误码语义**：BusinessError panic → `code: 500` + 业务 errCode；
  其他 panic → `code: 531`。与 beego vanilla 语义一致。

## 作为下游项目的起点

1. 拷贝本目录为新项目
2. 删除 go.mod 中的 `replace github.com/suna0/carina => ../`
3. `go get github.com/suna0/carina@v0.2.0 && go mod tidy`
4. 全局替换 module 名 `example` 为你的服务名
