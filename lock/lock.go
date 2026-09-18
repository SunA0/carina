package lock

import (
	"time"

	"github.com/go-redsync/redsync/v4"
	redsyncgoredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredis "github.com/redis/go-redis/v9"
)

// Mutex 分布式锁句柄，Unlock 释放
type Mutex = redsync.Mutex

// ILock 锁引擎接口
type ILock interface {
	Lock(key string, opts ...*LockOption) (*Mutex, error)
}

// ---- DummyLock 空实现（开发环境） ----

type DummyLock struct{}

func (d *DummyLock) Lock(key string, opts ...*LockOption) (*Mutex, error) {
	return nil, nil
}

// ---- RedisLock 基于 redsync 的实现 ----

type RedisLock struct {
	engine *redsync.Redsync
}

func (r *RedisLock) Lock(key string, opts ...*LockOption) (*Mutex, error) {
	opt := NewLockOption(key)
	if len(opts) > 0 && opts[0] != nil {
		opt = opts[0]
		if opt.Key == "" {
			opt.Key = key
		}
	}
	mutex := r.engine.NewMutex(opt.Key,
		redsync.WithExpiry(time.Duration(opt.Timeout)*time.Second),
		redsync.WithTries(opt.Tries),
	)
	if err := mutex.Lock(); err != nil {
		return nil, err
	}
	return mutex, nil
}

// ---- 全局锁实例 ----

var global ILock = &DummyLock{}

// InitRedis 使用 go-redis 客户端初始化分布式锁
func InitRedis(client *goredis.Client) {
	pool := redsyncgoredis.NewPool(client)
	global = &RedisLock{engine: redsync.New(pool)}
}

// InitDummy 使用空锁（默认，开发环境）
func InitDummy() {
	global = &DummyLock{}
}

// Lock 使用全局锁引擎加锁
func Lock(key string, opts ...*LockOption) (*Mutex, error) {
	return global.Lock(key, opts...)
}
