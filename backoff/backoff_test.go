package backoff

import (
	"errors"
	"testing"
	"time"
)

// TestConstantBackOff 固定间隔
func TestConstantBackOff(t *testing.T) {
	b := NewConstantBackOff(time.Second)
	if b.NextBackOff() != time.Second {
		t.Fatal("constant backoff should always return interval")
	}
}

// TestZeroAndStop 零/停止策略
func TestZeroAndStop(t *testing.T) {
	z := &ZeroBackOff{}
	if z.NextBackOff() != 0 {
		t.Fatal("zero backoff should return 0")
	}
	s := &StopBackOff{}
	if s.NextBackOff() != Stop {
		t.Fatal("stop backoff should return Stop")
	}
}

// TestExponentialBackOffGrow 指数退避间隔增长且受 MaxInterval 封顶
func TestExponentialBackOffGrow(t *testing.T) {
	b := NewExponentialBackOff()
	b.RandomizationFactor = 0 // 关闭随机，结果确定
	b.InitialInterval = 100 * time.Millisecond
	b.Multiplier = 2
	b.MaxInterval = 250 * time.Millisecond
	b.MaxElapsedTime = 0
	b.Reset()

	if d := b.NextBackOff(); d != 100*time.Millisecond {
		t.Fatalf("first=%v want 100ms", d)
	}
	if d := b.NextBackOff(); d != 200*time.Millisecond {
		t.Fatalf("second=%v want 200ms", d)
	}
	if d := b.NextBackOff(); d != 250*time.Millisecond {
		t.Fatalf("third=%v want capped 250ms", d)
	}
}

// TestRetrySuccess 重试直到成功
func TestRetrySuccess(t *testing.T) {
	attempts := 0
	err := Retry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("fail")
		}
		return nil
	}, NewConstantBackOff(time.Millisecond))
	if err != nil {
		t.Fatalf("retry should succeed, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// TestRetryStop 策略停止后返回最后错误
func TestRetryStop(t *testing.T) {
	attempts := 0
	err := Retry(func() error {
		attempts++
		return errors.New("always fail")
	}, &StopBackOff{})
	if err == nil {
		t.Fatal("retry should return error after stop")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}
