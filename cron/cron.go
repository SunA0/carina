package cron

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/suna0/carina/db"
	"github.com/suna0/carina/tracing"
	"gorm.io/gorm"

	"github.com/robfig/cron/v3"
)

// TaskInterface 定时任务接口
type TaskInterface interface {
	// Run 执行任务逻辑
	Run(ctx *TaskContext) error
	// GetName 返回任务名称
	GetName() string
	// IsEnableTx 是否在事务中执行
	IsEnableTx() bool
}

// Task 定时任务基类
type Task struct {
	name string
}

func (t *Task) Run(ctx *TaskContext) error { return fmt.Errorf("Run not implemented") }
func (t *Task) GetName() string            { return t.name }
func (t *Task) IsEnableTx() bool           { return true }

// NewTask 创建任务基类
func NewTask(name string) Task {
	return Task{name: name}
}

// TaskContext 定时任务上下文
type TaskContext struct {
	Ctx context.Context
	db  *gorm.DB
}

// DB 获取数据库句柄
func (tc *TaskContext) DB() *gorm.DB { return tc.db }

// Context 获取 context
func (tc *TaskContext) Context() context.Context { return tc.Ctx }

// CronTask 已注册的定时任务
type CronTask struct {
	name     string
	spec     string
	taskFunc cron.FuncJob
	onlyRun  bool
}

// OnlyRun 标记此任务为唯一运行任务
func (t *CronTask) OnlyRun() {
	t.onlyRun = true
}

var name2task = make(map[string]*CronTask)

// taskWrapper 包装任务执行，统一处理事务、Recovery、日志
func taskWrapper(task TaskInterface) cron.FuncJob {
	return func() {
		taskName := task.GetName()
		ctx := context.Background()

		// 创建 tracing span
		ctx, span := tracing.StartSpan(ctx, fmt.Sprintf("cron.%s", taskName))
		defer span.End()

		var tx *gorm.DB
		if task.IsEnableTx() {
			var err error
			ctx, tx, err = db.BeginTx(ctx)
			if err != nil {
				log.Printf("[cron] %s: begin tx failed: %v", taskName, err)
				return
			}
		}

		defer func() {
			if err := recover(); err != nil {
				if tx != nil {
					db.RollbackTx(ctx)
				}
				log.Printf("[cron] %s: panic: %v", taskName, err)
			}
		}()

		taskCtx := &TaskContext{Ctx: ctx, db: db.GetDBWithContext(ctx)}

		startTime := time.Now()
		log.Printf("[cron] %s: run...", taskName)

		err := task.Run(taskCtx)

		elapsed := time.Since(startTime)
		if err != nil {
			if tx != nil {
				db.RollbackTx(ctx)
			}
			log.Printf("[cron] %s: failed after %v: %v", taskName, elapsed, err)
		} else {
			if tx != nil {
				db.CommitTx(ctx)
			}
			log.Printf("[cron] %s: done, cost %v", taskName, elapsed)
		}
	}
}

// RegisterTask 注册定时任务
// spec 为 cron 表达式，如 "0 0 * * * *"（秒级精度）
func RegisterTask(task TaskInterface, spec string) *CronTask {
	cronTask := &CronTask{
		name:     task.GetName(),
		spec:     spec,
		taskFunc: taskWrapper(task),
	}
	name2task[task.GetName()] = cronTask
	return cronTask
}

// RegisterFunc 注册函数式定时任务
func RegisterFunc(name string, spec string, fn func() error) *CronTask {
	task := &funcTask{name: name, fn: fn}
	return RegisterTask(task, spec)
}

// funcTask 函数式任务适配器
type funcTask struct {
	Task
	name string
	fn   func() error
}

func (t *funcTask) GetName() string { return t.name }
func (t *funcTask) Run(ctx *TaskContext) error {
	if t.fn != nil {
		return t.fn()
	}
	return nil
}

// StartCronTasks 启动所有注册的定时任务
func StartCronTasks() {
	var onlyRun *CronTask
	for _, t := range name2task {
		if t.onlyRun {
			onlyRun = t
		}
	}

	c := cron.New(cron.WithSeconds())

	if onlyRun != nil {
		log.Printf("[cron] create task (only-run): %s %s", onlyRun.name, onlyRun.spec)
		c.AddJob(onlyRun.spec, onlyRun.taskFunc)
	} else {
		for _, t := range name2task {
			log.Printf("[cron] create task: %s %s", t.name, t.spec)
			c.AddJob(t.spec, t.taskFunc)
		}
	}

	c.Start()
	log.Printf("[cron] started %d tasks", len(name2task))
}

// StopCronTasks 停止所有定时任务
func StopCronTasks() {
	// robfig/cron v3 的 Stop 不直接暴露，需要持有 cron 实例
	// 简化实现：进程退出即停止
	log.Println("[cron] stop tasks (process exit)")
}
