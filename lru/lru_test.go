package lru

import (
	"testing"
	"time"
)

// TestSetGet 基本读写
func TestSetGet(t *testing.T) {
	c := NewCache("test", 2)
	if c == nil {
		t.Fatal("cache should not be nil")
	}
	c.Set("a", 1)
	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatalf("Get(a)=%v,%v want 1,true", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get(missing) should be false")
	}
}

// TestEvict 超容量淘汰最久未使用项
func TestEvict(t *testing.T) {
	c := NewCache("test", 2)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Get("a") // a 变热，b 成为最久未使用
	c.Set("c", 3)

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should survive")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("c should exist")
	}
}

// TestTTL 过期自动移除
func TestTTL(t *testing.T) {
	c := NewCache("test", 10, WithTTL(50*time.Millisecond))
	c.Set("a", 1)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should exist before expiry")
	}
	time.Sleep(80 * time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should expire")
	}
}

// TestDel 删除
func TestDel(t *testing.T) {
	c := NewCache("test", 2)
	c.Set("a", 1)
	if !c.Del("a") {
		t.Fatal("Del(a) should be true")
	}
	if c.Del("a") {
		t.Fatal("Del(a) again should be false")
	}
}

// TestEvictCallback 淘汰回调触发
func TestEvictCallback(t *testing.T) {
	evicted := make(chan interface{}, 1)
	c := NewCache("test", 1, WithEvictCallBack(func(k, v interface{}) {
		evicted <- k
	}))
	c.Set("a", 1)
	c.Set("b", 2) // 淘汰 a
	select {
	case k := <-evicted:
		if k != "a" {
			t.Fatalf("evicted key=%v want a", k)
		}
	case <-time.After(time.Second):
		t.Fatal("evict callback not called")
	}
}

// TestNewCacheInvalid 非法参数返回 nil
func TestNewCacheInvalid(t *testing.T) {
	if NewCache("bad", 0) != nil {
		t.Fatal("cap=0 should return nil")
	}
	if NewCache("bad", 1, WithTTL(-time.Second)) != nil {
		t.Fatal("negative ttl should return nil")
	}
}
