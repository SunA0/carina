package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// Options Redis 连接选项
type Options struct {
	Address  string
	Password string
	DB       int
}

// Init 初始化全局 Redis 客户端
func Init(opts Options) error {
	client = redis.NewClient(&redis.Options{
		Addr:     opts.Address,
		Password: opts.Password,
		DB:       opts.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cache: ping redis: %w", err)
	}
	return nil
}

// Client 返回全局 *redis.Client，未初始化时 panic
func Client() *redis.Client {
	if client == nil {
		panic("cache: not initialized, call cache.Init first")
	}
	return client
}

// Get 获取缓存值
func Get(ctx context.Context, key string) (string, error) {
	return Client().Get(ctx, key).Result()
}

// Set 设置缓存值
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return Client().Set(ctx, key, value, expiration).Err()
}

// SetEx 设置带过期时间的缓存
func SetEx(ctx context.Context, key string, value interface{}, seconds int) error {
	return Client().Set(ctx, key, value, time.Duration(seconds)*time.Second).Err()
}

// Delete 删除缓存
func Delete(ctx context.Context, keys ...string) error {
	return Client().Del(ctx, keys...).Err()
}

// Exists 检查 key 是否存在
func Exists(ctx context.Context, keys ...string) (int64, error) {
	return Client().Exists(ctx, keys...).Result()
}

// Incr 自增
func Incr(ctx context.Context, key string) (int64, error) {
	return Client().Incr(ctx, key).Result()
}

// Decr 自减
func Decr(ctx context.Context, key string) (int64, error) {
	return Client().Decr(ctx, key).Result()
}

// HSet 设置哈希字段
func HSet(ctx context.Context, key string, values ...interface{}) error {
	return Client().HSet(ctx, key, values...).Err()
}

// HGet 获取哈希字段
func HGet(ctx context.Context, key, field string) (string, error) {
	return Client().HGet(ctx, key, field).Result()
}

// HGetAll 获取哈希所有字段
func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return Client().HGetAll(ctx, key).Result()
}

// MGet 批量获取
func MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return Client().MGet(ctx, keys...).Result()
}

// ClearByPrefix 按前缀清除缓存
func ClearByPrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	for {
		keys, nextCursor, err := Client().Scan(ctx, cursor, prefix+":*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := Client().Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// IsInitialized 返回是否已初始化
func IsInitialized() bool {
	return client != nil
}
