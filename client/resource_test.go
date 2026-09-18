package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// countingListener 统计服务端接受的 TCP 连接数，用于判断客户端是否复用连接
type countingListener struct {
	net.Listener
	accepts *int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	atomic.AddInt64(l.accepts, 1)
	return l.Listener.Accept()
}

// newBadServer 返回总是产生错误的测试服务（响应体非法 JSON → 调用方视为失败）
func newBadServer(hits *int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		_, _ = w.Write([]byte("not-json"))
	}))
}

// TestPostNotRetriedOnFailure 非幂等 POST 失败时默认重试策略会导致重复提交。
// 预期：POST/PUT/DELETE 等写操作不应自动重试（服务端可能已处理成功），
// 若失败说明默认重试对非幂等方法有风险。
func TestPostNotRetriedOnFailure(t *testing.T) {
	var hits int64
	server := newBadServer(&hits)
	defer server.Close()

	Init("unit-test", "test", strings.TrimPrefix(server.URL, "http://"))

	_, err := NewResource(context.Background()).Post("svc", "some.res", Map{"k": "v"})
	if err == nil {
		t.Fatal("期望解析错误")
	}
	if n := atomic.LoadInt64(&hits); n != 1 {
		t.Errorf("BUG: 非幂等 POST 被自动重试 %d 次，可能重复写入下游", n)
	}
}

// TestGetRetried GET 幂等请求失败可重试
func TestGetRetried(t *testing.T) {
	var hits int64
	server := newBadServer(&hits)
	defer server.Close()

	Init("unit-test", "test", strings.TrimPrefix(server.URL, "http://"))

	_, err := NewResource(context.Background()).Get("svc", "some.res", Map{"k": "v"})
	if err == nil {
		t.Fatal("期望解析错误")
	}
	if n := atomic.LoadInt64(&hits); n != defaultRetryCount {
		t.Errorf("GET 重试次数=%d want %d", n, defaultRetryCount)
	}
}

// TestDisableRetry DisableRetry 后任何方法都只调用一次
func TestDisableRetry(t *testing.T) {
	var hits int64
	server := newBadServer(&hits)
	defer server.Close()

	Init("unit-test", "test", strings.TrimPrefix(server.URL, "http://"))

	_, err := NewResource(context.Background()).DisableRetry().Post("svc", "some.res", nil)
	if err == nil {
		t.Fatal("期望解析错误")
	}
	if n := atomic.LoadInt64(&hits); n != 1 {
		t.Errorf("DisableRetry 后调用次数=%d want 1", n)
	}
}

// TestConcurrentResourceRequests 多 goroutine 并发使用独立 Resource（配合 -race），
// 验证客户端在并发场景下无共享状态竞争。
func TestConcurrentResourceRequests(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		_, _ = w.Write([]byte(`{"code":200,"data":{"ok":true}}`))
	}))
	defer server.Close()

	Init("unit-test", "test", strings.TrimPrefix(server.URL, "http://"))

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			resp, err := NewResource(context.Background()).Get("svc", "some.res", Map{"i": 1})
			done <- err == nil && resp.IsSuccess()
		}()
	}
	for i := 0; i < 20; i++ {
		if !<-done {
			t.Error("并发请求失败")
		}
	}
}

// TestSharedTransportReuse 检查同一 host 的连续请求是否复用连接。
// 预期：keep-alive 下两次串行请求只需 1 个 TCP 连接。
// 若失败说明 client.request 每请求新建 http.Transport，
// 连接无法复用且空闲 Transport 从不关闭，高 QPS 下存在 TIME_WAIT 堆积/泄漏风险。
func TestSharedTransportReuse(t *testing.T) {
	var accepts int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	server.Listener = &countingListener{Listener: server.Listener, accepts: &accepts}
	server.Start()
	defer server.Close()

	Init("unit-test", "test", strings.TrimPrefix(server.URL, "http://"))
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := NewResource(ctx).Get("svc", "some.res", nil); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	if n := atomic.LoadInt64(&accepts); n > 1 {
		t.Errorf("BUG: 连续请求未复用连接，服务端新建连接数=%d，want 1（每请求新建 Transport）", n)
	}
}
