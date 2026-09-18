package db

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// BeginTx 在 context 中开启新事务，返回注入事务后的 context 和 *gorm.DB。
// 调用方负责在结束时调用返回的 commit/rollback 函数。
func BeginTx(ctx context.Context) (context.Context, *gorm.DB, error) {
	db := GetDB().WithContext(ctx)
	tx := db.Begin()
	if tx.Error != nil {
		return ctx, nil, fmt.Errorf("db: begin tx: %w", tx.Error)
	}
	return ContextWithTx(ctx, tx), tx, nil
}

// CommitTx 提交 context 中的事务（若存在）
func CommitTx(ctx context.Context) error {
	tx := TxFromContext(ctx)
	if tx == nil {
		return nil
	}
	return tx.Commit().Error
}

// RollbackTx 回滚 context 中的事务（若存在）
func RollbackTx(ctx context.Context) error {
	tx := TxFromContext(ctx)
	if tx == nil {
		return nil
	}
	return tx.Rollback().Error
}
