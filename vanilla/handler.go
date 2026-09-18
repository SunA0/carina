package vanilla

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/db"
	"github.com/suna0/carina/lock"
	"github.com/suna0/carina/metrics"
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
var metaCache = make(map[string]*resourceMeta)

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

// CreateHandler 创建 Gin handler，处理反射派发、参数校验、事务和锁。
//
// 并发安全：每请求通过 reflect.New 创建独立实例，不存在共享可变状态。
func CreateHandler(prototype RestResourceInterface) gin.HandlerFunc {
	resource := prototype.Resource()

	// 启动时预计算元数据
	meta, ok := metaCache[resource]
	if !ok {
		meta = precomputeMeta(prototype)
		metaCache[resource] = meta
	}

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
		if !disableTx {
			txCtx, tx, err := db.BeginTx(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusOK, MakeErrorResponse(500, "rest:tx_begin_failed",
					fmt.Sprintf("begin transaction failed: %v", err)))
				return
			}
			c.Request = c.Request.WithContext(txCtx)
			instance.InitContext(c) // 更新 context

			// PrepareOrm 钩子
			instance.PrepareOrm(tx)

			defer func() {
				if r := recover(); r != nil {
					db.RollbackTx(c.Request.Context())
					panic(r)
				}
			}()
		}

		// 4. 调用 Prepare
		instance.Prepare()

		// 5. 记录 metrics
		start := time.Now()
		if metrics.EndpointCounter != nil {
			metrics.EndpointCounter.WithLabelValues(resource, method).Inc()
		}

		// 6. 派发到具体方法
		methodHandler, exists := meta.methods[method]
		if !exists {
			c.JSON(http.StatusMethodNotAllowed, MakeErrorResponse(405, "rest:method_not_allowed",
				fmt.Sprintf("method %s not allowed", method)))
			return
		}
		methodHandler.Func.Call([]reflect.Value{reflect.ValueOf(instance)})

		// 7. 提交事务
		if !disableTx {
			if cb := GetBeforeCommitCallback(); cb != nil {
				cb(c.Request.Context())
			}
			if err := db.CommitTx(c.Request.Context()); err != nil {
				c.JSON(http.StatusOK, MakeErrorResponse(500, "rest:tx_commit_failed",
					fmt.Sprintf("commit transaction failed: %v", err)))
				return
			}

			// AfterCommitCallback
			if embedded != nil && embedded.AfterCommitCallback != nil {
				embedded.AfterCommitCallback()
			}
		}

		// 8. Finish
		instance.Finish()

		// 9. 记录耗时
		if metrics.EndpointDuration != nil {
			metrics.EndpointDuration.WithLabelValues(resource, method).Observe(time.Since(start).Seconds())
		}
	}
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
