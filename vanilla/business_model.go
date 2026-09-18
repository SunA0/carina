package vanilla

import (
	"context"

	"github.com/suna0/carina/db"
	"gorm.io/gorm"
)

// bContextKey 业务上下文在 gin.Context 中的 key
const bContextKey = "bContext"

// GetDBFromContext 从 context 获取 DB（优先事务）
func GetDBFromContext(ctx context.Context) *gorm.DB {
	return db.GetDBWithContext(ctx)
}

// GetOrmFromContext 兼容 vanilla 命名，获取 ORM
// Deprecated: 使用 GetDBFromContext
func GetOrmFromContext(ctx context.Context) *gorm.DB {
	return db.GetDBWithContext(ctx)
}

// RepositoryBase 数据访问层基类
type RepositoryBase struct {
	Ctx context.Context
}

// DB 获取数据库句柄（自动感知事务）
func (r *RepositoryBase) DB() *gorm.DB {
	return db.GetDBWithContext(r.Ctx)
}

// ServiceBase 业务逻辑层基类
type ServiceBase struct {
	Ctx context.Context
}

// DB 获取数据库句柄（自动感知事务）
func (s *ServiceBase) DB() *gorm.DB {
	return db.GetDBWithContext(s.Ctx)
}

// EntityBase 实体基类
type EntityBase struct {
	Ctx   context.Context
	Model interface{}
}

// IBusinessContextFactory 业务上下文工厂接口。
// 项目实现此接口，决定哪些字段注入到 context 中。
type IBusinessContextFactory interface {
	NewContext(ctx context.Context, userId int, jwtToken string) context.Context
}

var globalBContextFactory IBusinessContextFactory

// SetBusinessContextFactory 设置全局业务上下文工厂
func SetBusinessContextFactory(factory IBusinessContextFactory) {
	globalBContextFactory = factory
}

// GetBusinessContextFactory 获取全局业务上下文工厂
func GetBusinessContextFactory() IBusinessContextFactory {
	return globalBContextFactory
}

// BContextKey 返回 gin.Context 中业务上下文的 key
func BContextKey() string {
	return bContextKey
}
