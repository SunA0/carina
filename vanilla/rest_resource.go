package vanilla

import (
	"context"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/lock"
	"gorm.io/gorm"
)

// RestResourceInterface 所有 REST 资源必须实现的接口。
// 该接口为框架契约，一旦发布 breaking change 需 major 版本。
type RestResourceInterface interface {
	// Resource 返回资源名（点分格式，如 "account.corp"）
	Resource() string
	// GetAlias 返回 URL 别名集合
	GetAlias() []string
	// GetParameters 声明各 HTTP Method 的参数，用于自动校验
	// 格式: map[METHOD][]string{"GET": {"?id:int", "name:string"}}
	//   "?" 前缀表示可选参数
	//   类型: string, int, float, bool, json, json-raw, json-array
	GetParameters() map[string][]string
	// DisableTx 是否关闭事务支持（默认 GET 关闭）
	DisableTx() bool
	// GetLockKey 获取分布式锁 key，空字符串表示不加锁
	GetLockKey() string
	// GetLockOption 获取锁选项，nil 表示使用默认
	GetLockOption() *lock.LockOption
	// PrepareOrm 在事务开启前的 ORM 准备钩子
	PrepareOrm(db *gorm.DB)
	// GetBusinessContext 获取业务上下文
	GetBusinessContext() context.Context
	// IsForDevTest 是否为开发测试资源
	IsForDevTest() bool
	// EnableHTMLResource 是否开启 HTML 资源
	EnableHTMLResource() bool

	// Gin 生命周期
	InitContext(c *gin.Context)
	Prepare()
	Finish()
}

// AfterCommitCallbackFunc 事务提交后回调
type AfterCommitCallbackFunc func()

// BeforeCommitCallbackFunc 事务提交前回调
type BeforeCommitCallbackFunc func(ctx context.Context)

var (
	beforeCommitMu        sync.RWMutex
	gBeforeCommitCallback BeforeCommitCallbackFunc
)

// SetBeforeCommitCallback 设置全局事务提交前回调
func SetBeforeCommitCallback(callback BeforeCommitCallbackFunc) {
	beforeCommitMu.Lock()
	defer beforeCommitMu.Unlock()
	gBeforeCommitCallback = callback
}

// GetBeforeCommitCallback 获取全局事务提交前回调
func GetBeforeCommitCallback() BeforeCommitCallbackFunc {
	beforeCommitMu.RLock()
	defer beforeCommitMu.RUnlock()
	return gBeforeCommitCallback
}

// RestResource 所有 REST 资源的基类。
// 业务资源内嵌此结构体，获得参数解析、事务管理、锁管理等能力。
type RestResource struct {
	Ctx *gin.Context

	// 请求数据（由参数解析填充）
	Name2JSON      map[string]map[string]interface{}
	Name2JSONArray map[string][]interface{}
	Name2RAWJSON   map[string]interface{}
	Filters        map[string]interface{}

	// AfterCommitCallback 事务提交后的回调
	AfterCommitCallback AfterCommitCallbackFunc

	// BusinessContext 业务上下文（由中间件注入）
	BusinessCtx context.Context
}

// InitContext 初始化请求上下文
func (r *RestResource) InitContext(c *gin.Context) {
	r.Ctx = c
	r.Name2JSON = make(map[string]map[string]interface{})
	r.Name2JSONArray = make(map[string][]interface{})
	r.Name2RAWJSON = make(map[string]interface{})
	r.Filters = make(map[string]interface{})
}

// Prepare 请求预处理（参数校验、锁获取、事务开启由 handler 统一调度）
func (r *RestResource) Prepare() {}

// Finish 请求后处理
func (r *RestResource) Finish() {}

// --- 默认接口实现 ---

func (r *RestResource) Resource() string                   { return "" }
func (r *RestResource) GetAlias() []string                 { return nil }
func (r *RestResource) GetParameters() map[string][]string { return nil }
func (r *RestResource) IsForDevTest() bool                 { return false }
func (r *RestResource) EnableHTMLResource() bool           { return false }
func (r *RestResource) GetLockKey() string                 { return "" }
func (r *RestResource) GetLockOption() *lock.LockOption    { return nil }
func (r *RestResource) PrepareOrm(db *gorm.DB)             {}

// DisableTx 默认 GET 请求关闭事务
func (r *RestResource) DisableTx() bool {
	if r.Ctx == nil {
		return true
	}
	return r.Ctx.Request.Method == http.MethodGet
}

// GetBusinessContext 获取业务上下文
func (r *RestResource) GetBusinessContext() context.Context {
	if r.BusinessCtx != nil {
		return r.BusinessCtx
	}
	if r.Ctx != nil {
		return r.Ctx.Request.Context()
	}
	return context.Background()
}

// --- 参数获取辅助方法 ---

// paramValue 统一参数取值：POST 表单优先，URL query 兜底（对齐 beego GetString 语义）
func paramValue(c *gin.Context, key string) string {
	if v := c.PostForm(key); v != "" {
		return v
	}
	return c.Query(key)
}

// GetString 获取 string 参数
func (r *RestResource) GetString(key string) string {
	return paramValue(r.Ctx, key)
}

// GetStringDefault 获取 string 参数，带默认值
func (r *RestResource) GetStringDefault(key, def string) string {
	if v := paramValue(r.Ctx, key); v != "" {
		return v
	}
	return def
}

// GetInt 获取 int 参数
func (r *RestResource) GetInt(key string) (int, bool) {
	v := paramValue(r.Ctx, key)
	if v == "" {
		return 0, false
	}
	var result int
	if _, err := parseInt(v, &result); err != nil {
		return 0, false
	}
	return result, true
}

// GetIntDefault 获取 int 参数，带默认值
func (r *RestResource) GetIntDefault(key string, def int) int {
	if v, ok := r.GetInt(key); ok {
		return v
	}
	return def
}

// GetInt64 获取 int64 参数
func (r *RestResource) GetInt64(key string) (int64, bool) {
	v := paramValue(r.Ctx, key)
	if v == "" {
		return 0, false
	}
	var result int64
	if _, err := parseInt64(v, &result); err != nil {
		return 0, false
	}
	return result, true
}

// GetFloat 获取 float64 参数
func (r *RestResource) GetFloat(key string) (float64, bool) {
	v := paramValue(r.Ctx, key)
	if v == "" {
		return 0, false
	}
	var result float64
	if _, err := parseFloat(v, &result); err != nil {
		return 0, false
	}
	return result, true
}

// GetBool 获取 bool 参数
func (r *RestResource) GetBool(key string) (bool, bool) {
	v := paramValue(r.Ctx, key)
	if v == "" {
		return false, false
	}
	return parseBool(v), true
}

// GetJSON 获取 JSON 对象参数
func (r *RestResource) GetJSON(key string) map[string]interface{} {
	if data, ok := r.Name2JSON[key]; ok {
		return data
	}
	return nil
}

// GetJSONArray 获取 JSON 数组参数
func (r *RestResource) GetJSONArray(key string) []interface{} {
	if data, ok := r.Name2JSONArray[key]; ok {
		return data
	}
	return nil
}

// GetIntArray 获取 int 数组参数
func (r *RestResource) GetIntArray(key string) []int {
	arr := r.GetJSONArray(key)
	if arr == nil {
		return nil
	}
	result := make([]int, 0, len(arr))
	for _, v := range arr {
		switch val := v.(type) {
		case float64:
			result = append(result, int(val))
		case string:
			var n int
			if _, err := parseInt(val, &n); err == nil {
				result = append(result, n)
			}
		}
	}
	return result
}

// GetStringArray 获取 string 数组参数
func (r *RestResource) GetStringArray(key string) []string {
	arr := r.GetJSONArray(key)
	if arr == nil {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// GetFilters 获取过滤器
func (r *RestResource) GetFilters() map[string]interface{} {
	return r.Filters
}

// --- 响应方法 ---

// ReturnJSON 返回统一 JSON 响应
func (r *RestResource) ReturnJSON(response *Response) {
	r.Ctx.JSON(http.StatusOK, response)
}

// ReturnJSONWithCallback 返回 JSON 并设置事务提交后回调
func (r *RestResource) ReturnJSONWithCallback(response *Response, callback AfterCommitCallbackFunc) {
	r.AfterCommitCallback = callback
	r.Ctx.JSON(http.StatusOK, response)
}

// ReturnError 返回错误响应（HTTP 200 + 错误码）
func (r *RestResource) ReturnError(code int32, errCode, errMsg string) {
	r.ReturnJSON(MakeErrorResponse(code, errCode, errMsg))
}

// 编译期断言：确保 RestResource 始终完整实现契约接口，
// 接口变更会在此处立即编译失败，防止静默 break。
var _ RestResourceInterface = (*RestResource)(nil)
