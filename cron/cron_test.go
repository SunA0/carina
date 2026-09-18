package cron

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWithRetryHonorsInterval 验证 WithRetry 的 IntervalMs 生效。
// 预期：两次重试之间等待 IntervalMs；若失败说明重试间隔未实现（空转立即重试）。
func TestWithRetryHonorsInterval(t *testing.T) {
	var calls int
	fn := func() error {
		calls++
		if calls < 3 {
			return errors.New("boom")
		}
		return nil
	}

	start := time.Now()
	err := WithRetry(RetryPolicy{MaxRetries: 3, IntervalMs: 50}, fn)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WithRetry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls=%d want 3", calls)
	}
	// 2 次重试间隔 × 50ms = 至少 100ms
	if elapsed < 100*time.Millisecond {
		t.Errorf("BUG: IntervalMs=%d 未生效，实际耗时 %v（重试之间无等待，失败依赖时会产生重试风暴）",
			50, elapsed)
	}
}

// TestWithRetryExhausted 重试次数耗尽后返回最后一次错误
func TestWithRetryExhausted(t *testing.T) {
	wantErr := errors.New("always fail")
	calls := 0
	err := WithRetry(RetryPolicy{MaxRetries: 2}, func() error {
		calls++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err=%v want %v", err, wantErr)
	}
	if calls != 3 { // 首次 + 2 次重试
		t.Fatalf("calls=%d want 3", calls)
	}
}

// TestPipeConcurrentProduceConsume 管道并发生产/消费安全性（配合 -race）。
func TestPipeConcurrentProduceConsume(t *testing.T) {
	p := NewPipe(20)

	const producers = 8
	const perProducer = 50

	var wg sync.WaitGroup
	// 消费者：持续取出直到无数据可取且所有生产者完成
	stop := make(chan struct{})
	var consumed int64
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
				for {
					select {
					case <-p.ch:
						atomic.AddInt64(&consumed, 1)
						continue
					default:
					}
					break
				}
			}
		}
	}()

	var produced int64
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perProducer; j++ {
				for {
					if p.AddData(j) == nil {
						atomic.AddInt64(&produced, 1)
						break
					}
					select {
					case <-p.ch: // 满了先帮助消费掉一条，避免死等
						atomic.AddInt64(&consumed, 1)
					default:
						time.Sleep(time.Millisecond)
					}
				}
			}
		}()
	}

	time.Sleep(300 * time.Millisecond) // 等消费者排空
	close(stop)
	wg.Wait()

	// 守恒式：生产数 == 消费数 + 管道剩余数
	if leftover := int64(len(p.ch)); produced != consumed+leftover {
		t.Errorf("数据不守恒: produced=%d consumed=%d leftover=%d", produced, consumed, leftover)
	}
	if produced != producers*perProducer {
		t.Errorf("produced=%d want %d", produced, producers*perProducer)
	}
}
