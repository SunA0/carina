package db

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	globalDB *gorm.DB
	once     sync.Once
	initErr  error
)

// Options 数据库连接池选项
type Options struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int // 分钟
}

// Init 初始化全局数据库连接，驱动无关。
// dialector 由调用方传入，例如 mysql.Open(dsn) / postgres.Open(dsn)。
// 必须在使用任何 DB 功能之前调用。
func Init(dialector gorm.Dialector, opts *Options) error {
	once.Do(func() {
		db, err := gorm.Open(dialector, &gorm.Config{})
		if err != nil {
			initErr = fmt.Errorf("db: open: %w", err)
			return
		}
		if opts != nil {
			sqlDB, err := db.DB()
			if err != nil {
				initErr = fmt.Errorf("db: get sql.DB: %w", err)
				return
			}
			if opts.MaxIdleConns > 0 {
				sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
			}
			if opts.MaxOpenConns > 0 {
				sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
			}
		}
		globalDB = db
	})
	return initErr
}

// GetDB 返回全局 *gorm.DB 实例。
// 未调用 Init 时会 panic。
func GetDB() *gorm.DB {
	if globalDB == nil {
		panic("db: not initialized, call db.Init first")
	}
	return globalDB
}

// txKey 事务 context key
type txKey struct{}

// TxKey 导出事务 context key（供中间件使用）
var TxKey = txKey{}

// ContextWithTx 将事务注入 context
func ContextWithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, TxKey, tx)
}

// TxFromContext 从 context 提取事务，无则返回 nil
func TxFromContext(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}
	if tx, ok := ctx.Value(TxKey).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return nil
}

// GetDBWithContext 优先返回 context 中的事务 DB，否则返回全局 DB。
// 业务代码通过此方法获取 DB 句柄，自动感知事务。
func GetDBWithContext(ctx context.Context) *gorm.DB {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return GetDB().WithContext(ctx)
}

// WithTransaction 在事务中执行 fn，自动处理 Begin/Commit/Rollback。
// fn 返回 error 时回滚，否则提交。
func WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	db := GetDB().WithContext(ctx)
	tx := db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("db: begin tx: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}

// ErrNotInitialized 未初始化错误
var ErrNotInitialized = errors.New("db: not initialized")
