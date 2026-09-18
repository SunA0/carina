package metrics

import (
	"sync"
	"testing"
)

// TestInitIdempotent 验证 Init 幂等：并发重复调用不得 panic
// （promauto 注册到默认 Registry，重复注册会 panic，sync.Once 必须挡住）。
func TestInitIdempotent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Init()
		}()
	}
	wg.Wait()

	if EndpointCounter == nil || LRUCacheCounter == nil || PanicCounter == nil {
		t.Fatal("Init 后指标变量不应为 nil")
	}

	// 指标可用性与并发写入安全
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				EndpointCounter.WithLabelValues("res", "GET").Inc()
				EndpointDuration.WithLabelValues("res", "GET").Observe(0.01)
				PanicCounter.Inc()
			}
		}()
	}
	wg.Wait()
}
