package lru

import (
	"sync"
	"testing"
	"time"
)

// TestDelStaleTimerDeletesReinsertedKey 验证 Del 后过期定时器是否误删同 key 的新条目。
// 预期：Del 时定时器应被停止，重新写入的条目不受旧定时器影响。
// 若失败说明 removeEntry 未 Stop 定时器（陈旧定时器按 key 盲删 map）。
func TestDelStaleTimerDeletesReinsertedKey(t *testing.T) {
	c := NewCache("test", 4, WithTTL(50*time.Millisecond))

	c.Set("k", 1)
	c.Del("k")
	c.Set("k", 2) // 重新写入同 key

	time.Sleep(150 * time.Millisecond) // 等旧定时器触发

	if v, ok := c.Get("k"); !ok {
		t.Errorf("BUG: Del 后陈旧定时器未停止，误删了重新写入的条目，Get(k)=miss，want 2,true")
	} else if v != 2 {
		t.Errorf("Get(k)=%v want 2", v)
	}
}

// TestPurgeStaleTimersCorruptList 验证 Purge 后陈旧定时器对 LRU 链表计数的破坏。
// 预期：Purge 应停止所有定时器；否则旧定时器触发时会对新链表执行 Remove，
// 导致 Len() 与实际条目数不一致，进而破坏容量淘汰判断。
func TestPurgeStaleTimersCorruptList(t *testing.T) {
	c := NewCache("test", 3, WithTTL(40*time.Millisecond))

	c.Set("a", 1)
	c.Set("b", 2)
	c.Purge()

	c.Set("x", 10)
	c.Set("y", 20)
	c.Set("z", 30)

	time.Sleep(150 * time.Millisecond) // 等 a/b 的陈旧定时器触发

	if got := c.Len(); got != 3 {
		t.Errorf("BUG: Purge 后陈旧定时器破坏链表计数，Len()=%d，want 3", got)
	}
}

// TestEvictKeepsCapInvariant 容量不变式：并发写入下条目数不得超过 cap。
func TestEvictKeepsCapInvariant(t *testing.T) {
	c := NewCache("test", 16, WithTTL(time.Second))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				c.Set(j, id)
				c.Get(j)
			}
		}(i)
	}
	wg.Wait()

	if got := c.Len(); got > 16 {
		t.Errorf("容量不变式被破坏：Len()=%d，cap=16", got)
	}
}

// TestConcurrentMixedOperations 混合并发读写压测（配合 -race 检测数据竞争）。
// 定时器触发（removeEntry）与 Set/Get/Del/Purge 并发是重点检测路径。
func TestConcurrentMixedOperations(t *testing.T) {
	c := NewCache("test", 32, WithTTL(5*time.Millisecond))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 写
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; ; j++ {
				select {
				case <-stop:
					return
				default:
				}
				c.Set(j%100, j)
			}
		}()
	}
	// 读
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; ; j++ {
				select {
				case <-stop:
					return
				default:
				}
				c.Get(j % 100)
				c.Len()
				c.Keys()
			}
		}()
	}
	// 删 + 定时器过期并发触发
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; ; j++ {
				select {
				case <-stop:
					return
				default:
				}
				c.Del(j % 100)
			}
		}()
	}

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
}
