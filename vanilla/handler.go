package vanilla

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/db"
	"github.com/suna0/carina/lock"
	"github.com/suna0/carina/metrics"
	"gorm.io/gorm"
)

// resourceMeta 预计算的资源元数据
type resourceMeta struct {
	resourceType reflect.Type
	methods      map[string]reflect.Method // method name → method
	parameters   map[string][]string       // method → params
	resource     string
	disableTx    map[string]bool // method → disableTx
	lockKey      string
	lockOption   *lock.LockOption
	embeddedPath []int // 内嵌 RestResource 字段的索引路径（空表示未内嵌基类）
}

// metaCache 预计算缓存（启动时构建）
var (
	metaCache   = make(map[string]*resourceMeta)
	metaCacheMu sync.Mutex
)

// httpToGoMethod HTTP 方法 → Go 方法名映射
var httpToGoMethod = map[string]string{
	http.MethodGet:     "Get",
	http.MethodPost:    "Post",
	http.MethodPut:     "Put",
	http.MethodDelete:  "Delete",
	http.MethodPatch:   "Patch",
	http.MethodHead:    "Head",
	http.MethodOptions: "Options",
}

// HandlerOptions 配置资源 handler；零值保持旧版响应时序。
type HandlerOptions struct {
	// BufferResponseUntilCommit 开启事务响应缓冲，提交成功后才输出普通响应。
	// 仅作用于 DisableTx() 为 false 的请求；不支持 Flush 或 Hijack。
	BufferResponseUntilCommit bool
	// MaxResponseBytes 限制缓冲正文大小；小于等于零时使用 1 MiB。
	MaxResponseBytes int
}

const managedTransactionKey = "carina:vanilla:managed_transaction"

// CreateHandler 创建 Gin handler，处理反射派发、参数校验、事务和锁。
//
// 并发安全：每请求通过 reflect.New 创建独立实例，不存在共享可变状态。
func CreateHandler(prototype RestResourceInterface) gin.HandlerFunc {
	return CreateHandlerWithOptions(prototype, HandlerOptions{})
}

// CreateHandlerWithOptions 创建带选项的资源 handler，不改变资源接口。
func CreateHandlerWithOptions(prototype RestResourceInterface, opts HandlerOptions) gin.HandlerFunc {
	resource := prototype.Resource()

	// 启动时预计算元数据
	meta := cachedResourceMeta(prototype, resource)

	return func(c *gin.Context) {
		// 每请求创建独立实例
		instance := reflect.New(meta.resourceType).Interface().(RestResourceInterface)
		// 内嵌基类指针（须先于 InitContext 初始化，指针嵌入时下游可能不初始化；
		// 未内嵌时为 nil，参数校验/回调做降级处理）
		embedded := embeddedRestResource(instance, meta)
		instance.InitContext(c)

		method := c.Request.Method

		// 1. 参数解析 & 校验
		if !parseParameters(c, embedded, instance, method) {
			return // 校验失败已在 parseParameters 中返回响应
		}
		methodHandler, exists := meta.methods[method]
		if !exists {
			c.JSON(http.StatusMethodNotAllowed, MakeErrorResponse(405, "rest:method_not_allowed",
				fmt.Sprintf("method %s not allowed", method)))
			return
		}

		// 2. 分布式锁
		var mutex *lock.Mutex
		lockKey := instance.GetLockKey()
		if lockKey != "" {
			lockOpt := instance.GetLockOption()
			if lockOpt == nil {
				lockOpt = lock.NewLockOption(lockKey)
			}
			var err error
			mutex, err = lock.Lock(lockKey, lockOpt)
			if err != nil {
				c.JSON(http.StatusOK, MakeErrorResponse(500, "rest:acquire_lock_failed",
					fmt.Sprintf("acquire_lock_failed: %s", lockKey)))
				return
			}
			if mutex != nil {
				defer mutex.Unlock()
			}
		}

		// 3. 事务管理
		disableTx := instance.DisableTx()
		var txCtx context.Context
		var response *transactionResponse
		originalWriter := c.Writer
		errorCount := len(c.Errors)
		transactionDone := false
		if !disableTx {
			if opts.BufferResponseUntilCommit && originalWriter.Written() {
				// 上游已输出的响应无法撤回，禁止继续执行事务业务。
				log.Printf("vanilla: cannot buffer transaction response: response already written")
				c.Abort()
				return
			}
			var err error
			var tx *gorm.DB
			txCtx, tx, err = db.BeginTx(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusOK, MakeErrorResponse(500, "rest:tx_begin_failed",
					fmt.Sprintf("begin transaction failed: %v", err)))
				return
			}
			c.Request = c.Request.WithContext(txCtx)
			c.Set(managedTransactionKey, tx)
			if opts.BufferResponseUntilCommit {
				response = newTransactionResponse(originalWriter, opts.MaxResponseBytes)
				c.Writer = response
			}
			// Begin 成功后立即保护所有钩子及提前退出路径。
			defer func() {
				r := recover()
				c.Writer = originalWriter
				if !transactionDone {
					_ = db.RollbackTx(txCtx)
				}
				if r != nil {
					panic(r)
				}
			}()

			// 更新自定义 InitContext 缓存的上下文，同时保留参数解析结果。
			if embedded != nil {
				json, arrays, raw, filters := embedded.Name2JSON, embedded.Name2JSONArray, embedded.Name2RAWJSON, embedded.Filters
				instance.InitContext(c)
				embedded.Name2JSON, embedded.Name2JSONArray, embedded.Name2RAWJSON, embedded.Filters = json, arrays, raw, filters
			} else {
				instance.InitContext(c)
			}
			instance.PrepareOrm(tx)
		}

		// 4. 调用 Prepare
		instance.Prepare()

		// 5. 记录 metrics
		start := time.Now()
		if metrics.EndpointCounter != nil {
			metrics.EndpointCounter.WithLabelValues(resource, method).Inc()
		}

		// 6. 派发到具体方法
		methodHandler.Func.Call([]reflect.Value{reflect.ValueOf(instance)})

		// 7. 提交事务
		if !disableTx {
			if response != nil && responseFailed(c, response, errorCount) {
				return
			}
			if cb := GetBeforeCommitCallback(); cb != nil {
				cb(txCtx)
			}
			if response != nil && responseFailed(c, response, errorCount) {
				return
			}
			if err := db.CommitTx(txCtx); err != nil {
				_ = db.RollbackTx(txCtx) // 尽力清理；不能据此确认提交结果。
				transactionDone = true
				c.Writer = originalWriter
				c.JSON(http.StatusOK, MakeErrorResponse(500, "rest:tx_commit_failed",
					fmt.Sprintf("commit transaction failed: %v", err)))
				return
			}
			transactionDone = true
			if response != nil {
				response.seal()
			}

			// AfterCommitCallback
			if embedded != nil && embedded.AfterCommitCallback != nil {
				if response != nil {
					runAfterCommitHook(c, embedded.AfterCommitCallback)
				} else {
					embedded.AfterCommitCallback()
				}
			}
		}

		// 8. Finish
		if response != nil {
			runAfterCommitHook(c, instance.Finish)
			c.Writer = originalWriter
			if err := response.publish(); err != nil {
				// 事务已提交，网络写入失败不能追加第二份响应或撤销事务。
				_ = c.Error(fmt.Errorf("vanilla: publish transaction response: %w", err))
				log.Printf("vanilla: publish transaction response: %v", err)
			}
		} else {
			instance.Finish()
		}

		// 9. 记录耗时
		if metrics.EndpointDuration != nil {
			metrics.EndpointDuration.WithLabelValues(resource, method).Observe(time.Since(start).Seconds())
		}
	}
}

// responseFailed 将序列化、缓冲和本请求新增的 Gin 错误转为唯一错误响应。
func responseFailed(c *gin.Context, response *transactionResponse, errorCount int) bool {
	err := response.err
	if err == nil && len(c.Errors) > errorCount {
		err = c.Errors.Last().Err
	}
	if err == nil {
		return false
	}
	c.Writer = response.target
	c.AbortWithStatusJSON(http.StatusOK, MakeErrorResponse(500, "rest:response_failed", err.Error()))
	return true
}

// runAfterCommitHook 隔离提交后钩子的异常，不改变已提交请求的响应。
func runAfterCommitHook(c *gin.Context, hook func()) {
	errorCount := len(c.Errors)
	defer func() {
		if err := recover(); err != nil {
			_ = c.Error(fmt.Errorf("vanilla: after commit hook: %v", err))
			if metrics.PanicCounter != nil {
				metrics.PanicCounter.Inc()
			}
			logPanic(c, err)
		} else if len(c.Errors) > errorCount {
			log.Printf("vanilla: after commit hook: %v", c.Errors.Last().Err)
		}
	}()
	hook()
}

func cachedResourceMeta(prototype RestResourceInterface, resource string) *resourceMeta {
	metaCacheMu.Lock()
	defer metaCacheMu.Unlock()
	if meta, ok := metaCache[resource]; ok {
		return meta
	}
	meta := precomputeMeta(prototype)
	metaCache[resource] = meta
	return meta
}

// precomputeMeta 启动时预计算资源元数据
func precomputeMeta(prototype RestResourceInterface) *resourceMeta {
	resourceType := reflect.TypeOf(prototype).Elem()

	meta := &resourceMeta{
		resourceType: resourceType,
		methods:      make(map[string]reflect.Method),
		parameters:   prototype.GetParameters(),
		resource:     prototype.Resource(),
		disableTx:    make(map[string]bool),
		lockKey:      prototype.GetLockKey(),
		lockOption:   prototype.GetLockOption(),
	}

	// 查找内嵌的 RestResource 字段（支持值嵌入 / 指针嵌入 / 多层提升）
	if field, found := resourceType.FieldByName("RestResource"); found && field.Anonymous {
		meta.embeddedPath = field.Index
		// 防御性初始化 prototype 的内嵌指针（约定基类默认实现 nil 安全，此处保底）
		embeddedRestResource(prototype, meta)
	}

	// 预计算方法句柄（业务方法定义在指针接收者上，必须用指针类型查方法集）
	// key 为 HTTP 方法（大写），查找时用 Title 形式的 Go 方法名
	ptrType := reflect.PtrTo(resourceType)
	for httpMethod, goMethod := range httpToGoMethod {
		if method, found := ptrType.MethodByName(goMethod); found {
			meta.methods[httpMethod] = method
		}
	}

	return meta
}

// embeddedRestResource 获取实例内嵌的 *RestResource。
// 值嵌入直接取地址；指针嵌入为 nil 时初始化；未内嵌返回 nil。
func embeddedRestResource(instance RestResourceInterface, meta *resourceMeta) *RestResource {
	if len(meta.embeddedPath) == 0 {
		return nil
	}

	v := reflect.ValueOf(instance)
	for _, idx := range meta.embeddedPath {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(idx)
	}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return v.Interface().(*RestResource)
	}
	return v.Addr().Interface().(*RestResource)
}
