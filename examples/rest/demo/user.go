// Package demo 示例资源：演示 RestResource 的完整用法。
// 注册后自动获得：
//
//	GET/POST/PUT/DELETE /demo/user/
//	ANY                 /demo/api/user/
//	非 GET 请求自动包裹事务（DB 未初始化时本示例资源DisableTx）
//	GetLockKey() 非空时自动加分布式锁
package demo

import (
	"github.com/suna0/carina/vanilla"
)

// Greeting 包级配置注入点。
// 注意：框架每请求创建资源新实例，原型上的自定义字段不会传递，
// 项目侧配置请通过包级变量注入（由 main 在注册前赋值）。
var Greeting string

// User 示例资源
type User struct {
	vanilla.RestResource
}

// Resource 资源名（点分），决定路由路径 /demo/user/
func (r *User) Resource() string { return "demo.user" }

// DisableTx 本示例不依赖真实 DB，全方法关闭事务。
// 真实项目中删除本方法即恢复默认（GET 关闭，写方法开启）。
func (r *User) DisableTx() bool { return true }

// GetParameters 声明式参数校验。
// "?" 前缀可选；类型: string/int/float/bool/json/json-raw/json-array
func (r *User) GetParameters() map[string][]string {
	return map[string][]string{
		"GET":  {"?id:int", "?name:string"},
		"POST": {"name:string", "?profile:json", "?tags:json-array"},
	}
}

// Get 查询演示：参数获取 + Filter 协议 + 统一响应
func (r *User) Get() {
	id, _ := r.GetInt("id")
	name := r.GetStringDefault("name", "anonymous")

	// Filter 协议：GET /demo/user/?__f-age-gt=18&__f-name-contain=张
	// 业务层用法：
	//   db := vanilla.GetDBFromContext(r.GetBusinessContext())
	//   db = carinadb.ApplyFilters(db, r.GetFilters()).Find(&users)
	r.ReturnJSON(vanilla.MakeResponse(vanilla.Map{
		"id":       id,
		"name":     name,
		"greeting": Greeting,       // 包级变量注入的配置
		"filters":  r.GetFilters(), // 回显解析后的过滤器
	}))
}

// Post 创建演示：JSON 参数 + BusinessError panic 驱动错误流
func (r *User) Post() {
	name := r.GetString("name")
	if name == "admin" {
		// panic BusinessError → RecoverPanic 捕获 → 统一错误响应
		panic(vanilla.NewBusinessError("user:name_reserved", "该用户名被保留"))
	}

	profile := r.GetJSON("profile")  // map[string]interface{}
	tags := r.GetStringArray("tags") // []string

	r.ReturnJSON(vanilla.MakeResponse(vanilla.Map{
		"created": true,
		"name":    name,
		"profile": profile,
		"tags":    tags,
	}))
}

// Put 更新演示（可选方法，未实现的方法自动 405）
func (r *User) Put() {
	id, _ := r.GetInt("id")
	r.ReturnJSON(vanilla.MakeResponse(vanilla.Map{
		"updated": true,
		"id":      id,
	}))
}

// 编译期断言：确保实现完整契约
var _ vanilla.RestResourceInterface = (*User)(nil)
