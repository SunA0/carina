package cron

import "errors"

// ErrChannelFull 管道已满
var ErrChannelFull = errors.New("cron: channel is full")

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries int
	IntervalMs int
}

// DefaultRetryPolicy 默认重试策略
var DefaultRetryPolicy = RetryPolicy{
	MaxRetries: 3,
	IntervalMs: 1000,
}

// WithRetry 带重试执行函数
func WithRetry(policy RetryPolicy, fn func() error) error {
	var err error
	for i := 0; i <= policy.MaxRetries; i++ {
		if i > 0 {
			// 间隔重试
		}
		err = fn()
		if err == nil {
			return nil
		}
	}
	return err
}
