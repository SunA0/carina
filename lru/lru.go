// Package lru 提供带 TTL 的内存 LRU 缓存。
// 移植自 vanilla cache/lru，指标接入 carina/metrics（nil 安全）。
package lru

import (
	"container/list"
	"sync"
	"time"

	"github.com/suna0/carina/metrics"
)

// Cache LRU 缓存接口
type Cache interface {
	Set(key, value interface{}) bool
	Get(key interface{}) (interface{}, bool)
	Del(key interface{}) bool
	Keys() []interface{}
	Len() int
	Cap() int
	Purge()
}

type entry struct {
	key     interface{}
	value   interface{}
	element *list.Element
	expires time.Time
	timer   *time.Timer
}

// Option 缓存配置项
type Option func(*cache)

// EvictCallback 淘汰回调
type EvictCallback func(key interface{}, value interface{})

// WithTTL 设置过期时间（0 表示不过期）
func WithTTL(val time.Duration) Option {
	return func(c *cache) {
		c.ttl = val
	}
}

// WithEvictCallBack 设置淘汰回调
func WithEvictCallBack(callback EvictCallback) Option {
	return func(c *cache) {
		c.onEvict = callback
	}
}

// WithNoReset 设置 Get 时不刷新 TTL
func WithNoReset() Option {
	return func(c *cache) {
		c.noReset = true
	}
}

type cache struct {
	name      string
	cap       int
	ttl       time.Duration
	items     map[interface{}]*entry
	evictList *list.List
	lock      sync.RWMutex
	noReset   bool
	onEvict   EvictCallback
}

// NewCache 创建 LRU 缓存，cap 为容量上限。
// cap <= 0 或 ttl < 0 时返回 nil。
func NewCache(name string, cap int, opts ...Option) Cache {
	c := cache{cap: cap, name: name}

	for _, opt := range opts {
		opt(&c)
	}

	if c.cap <= 0 || c.ttl < 0 {
		return nil
	}

	c.items = make(map[interface{}]*entry, cap)
	c.evictList = list.New()
	return &c
}

func (c *cache) metric(op string) {
	if metrics.LRUCacheCounter != nil {
		metrics.LRUCacheCounter.WithLabelValues(c.name, op).Inc()
	}
}

// Set 写入缓存，返回是否发生了淘汰
func (c *cache) Set(key, value interface{}) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	// 已存在则更新
	if ent, ok := c.items[key]; ok {
		c.updateEntry(ent, value)
		return false
	}

	// 超容量则淘汰最久未使用的
	evict := c.evictList.Len() == c.cap
	if evict {
		if ele := c.evictList.Back(); ele != nil {
			ent := ele.Value.(*entry)
			c.metric("evict")
			c.removeEntry(ent)
		}
	}

	c.insertEntry(key, value)
	c.metric("set")
	return evict
}

func (c *cache) insertEntry(key, value interface{}) *entry {
	// 调用方必须已持有写锁
	ent := &entry{
		key:     key,
		value:   value,
		expires: time.Now().Add(c.ttl),
	}
	ele := c.evictList.PushFront(ent)
	ent.element = ele

	if c.ttl > 0 {
		ent.timer = time.AfterFunc(c.ttl, func() {
			c.lock.Lock()
			defer c.lock.Unlock()
			c.removeEntry(ent)
		})
	}

	c.items[key] = ent
	return ent
}

func (c *cache) updateEntry(e *entry, value interface{}) {
	// 调用方必须已持有写锁
	e.value = value
	c.renewEntry(e, true)
}

func (c *cache) resetEntryTTL(e *entry) {
	// 调用方必须已持有写锁
	if c.ttl > 0 {
		e.timer.Reset(c.ttl)
	}
	e.expires = time.Now().Add(c.ttl)
}

func (c *cache) renewEntry(e *entry, reset bool) {
	if reset {
		c.resetEntryTTL(e)
	}
	c.evictList.MoveToFront(e.element)
}

func (c *cache) removeEntry(e *entry) {
	// 调用方必须已持有写锁
	delete(c.items, e.key)
	if e.element != nil {
		c.evictList.Remove(e.element)
		e.element = nil // 避免内存泄漏
	}
	if c.onEvict != nil {
		c.onEvict(e.key, e.value)
	}
}

// Get 读取缓存
func (c *cache) Get(key interface{}) (interface{}, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		// 理论上过期项会被 timer 自动移除，防御性再检查一次
		if c.ttl == 0 || time.Now().Before(ent.expires) {
			c.renewEntry(ent, !c.noReset)
			c.metric("get-hit")
			return ent.value, true
		}
	}
	c.metric("get-miss")
	return nil, false
}

// Keys 返回所有未过期 key
func (c *cache) Keys() []interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()

	keys := make([]interface{}, 0, len(c.items))
	for k, v := range c.items {
		if c.ttl == 0 || time.Now().Before(v.expires) {
			keys = append(keys, k)
		}
	}
	return keys
}

// Len 当前条目数
func (c *cache) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.evictList.Len()
}

// Cap 容量上限
func (c *cache) Cap() int {
	return c.cap
}

// Purge 清空缓存
func (c *cache) Purge() {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, ent := range c.items {
		if c.onEvict != nil {
			c.onEvict(ent.key, ent.value)
		}
	}
	c.evictList.Init()
	c.items = make(map[interface{}]*entry, c.cap)
}

// Del 删除指定 key
func (c *cache) Del(key interface{}) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		c.removeEntry(ent)
		return true
	}
	return false
}
