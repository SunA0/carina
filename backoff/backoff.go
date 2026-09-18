// Package backoff 实现重试退避算法。
// 移植自 vanilla backoff（cenkalti/backoff 同款实现）。
package backoff

import (
	"math/rand"
	"time"
)

// BackOff 重试退避策略接口
type BackOff interface {
	// NextBackOff 返回下次重试前等待时长，返回 Stop 表示不再重试
	NextBackOff() time.Duration
	// Reset 重置到初始状态
	Reset()
}

// Stop 表示不再重试
const Stop time.Duration = -1

// ZeroBackOff 零等待策略：立即无限重试
type ZeroBackOff struct{}

func (b *ZeroBackOff) Reset()                     {}
func (b *ZeroBackOff) NextBackOff() time.Duration { return 0 }

// StopBackOff 永不重试策略
type StopBackOff struct{}

func (b *StopBackOff) Reset()                     {}
func (b *StopBackOff) NextBackOff() time.Duration { return Stop }

// ConstantBackOff 固定间隔策略
type ConstantBackOff struct {
	Interval time.Duration
}

func (b *ConstantBackOff) Reset()                     {}
func (b *ConstantBackOff) NextBackOff() time.Duration { return b.Interval }

// NewConstantBackOff 创建固定间隔策略
func NewConstantBackOff(d time.Duration) *ConstantBackOff {
	return &ConstantBackOff{Interval: d}
}

/*
ExponentialBackOff 指数退避策略，每次重试间隔按随机化函数指数增长。

NextBackOff() 计算公式：

	randomized interval =
	    RetryInterval * (random value in range [1 - RandomizationFactor, 1 + RandomizationFactor])

注意：实现非线程安全。
*/
type ExponentialBackOff struct {
	InitialInterval     time.Duration
	RandomizationFactor float64
	Multiplier          float64
	MaxInterval         time.Duration
	// MaxElapsedTime 超过该时长后 NextBackOff 返回 Stop；为 0 表示永不停止
	MaxElapsedTime time.Duration
	Clock          Clock

	currentInterval time.Duration
	startTime       time.Time
}

// Clock 时间接口（便于测试 mock）
type Clock interface {
	Now() time.Time
}

// ExponentialBackOff 默认参数
const (
	DefaultInitialInterval     = 500 * time.Millisecond
	DefaultRandomizationFactor = 0.5
	DefaultMultiplier          = 1.5
	DefaultMaxInterval         = 60 * time.Second
	DefaultMaxElapsedTime      = 15 * time.Minute
)

// NewExponentialBackOff 使用默认参数创建指数退避策略
func NewExponentialBackOff() *ExponentialBackOff {
	b := &ExponentialBackOff{
		InitialInterval:     DefaultInitialInterval,
		RandomizationFactor: DefaultRandomizationFactor,
		Multiplier:          DefaultMultiplier,
		MaxInterval:         DefaultMaxInterval,
		MaxElapsedTime:      DefaultMaxElapsedTime,
		Clock:               SystemClock,
	}
	b.Reset()
	return b
}

type systemClock struct{}

func (t systemClock) Now() time.Time {
	return time.Now()
}

// SystemClock 基于 time.Now() 的 Clock 实现
var SystemClock = systemClock{}

// Reset 重置间隔并重新开始计时
func (b *ExponentialBackOff) Reset() {
	b.currentInterval = b.InitialInterval
	b.startTime = b.Clock.Now()
}

// NextBackOff 计算下次退避间隔：
//
//	Randomized interval = RetryInterval +/- (RandomizationFactor * RetryInterval)
func (b *ExponentialBackOff) NextBackOff() time.Duration {
	if b.MaxElapsedTime != 0 && b.GetElapsedTime() > b.MaxElapsedTime {
		return Stop
	}
	defer b.incrementCurrentInterval()
	return getRandomValueFromInterval(b.RandomizationFactor, rand.Float64(), b.currentInterval)
}

// GetElapsedTime 返回自创建（或上次 Reset）以来的时长
func (b *ExponentialBackOff) GetElapsedTime() time.Duration {
	return b.Clock.Now().Sub(b.startTime)
}

func (b *ExponentialBackOff) incrementCurrentInterval() {
	if float64(b.currentInterval) >= float64(b.MaxInterval)/b.Multiplier {
		b.currentInterval = b.MaxInterval
	} else {
		b.currentInterval = time.Duration(float64(b.currentInterval) * b.Multiplier)
	}
}

func getRandomValueFromInterval(randomizationFactor, random float64, currentInterval time.Duration) time.Duration {
	var delta = randomizationFactor * float64(currentInterval)
	var minInterval = float64(currentInterval) - delta
	var maxInterval = float64(currentInterval) + delta

	return time.Duration(minInterval + (random * (maxInterval - minInterval + 1)))
}

// Operation 可重试操作，返回 error 表示失败（将按策略重试）
type Operation func() error

// Retry 按策略重试 operation，直到成功或策略停止。
// 返回 operation 的最后一次 error，成功返回 nil。
func Retry(operation Operation, b BackOff) error {
	var err error
	for {
		if err = operation(); err == nil {
			return nil
		}
		next := b.NextBackOff()
		if next == Stop {
			return err
		}
		time.Sleep(next)
	}
}

// RetryNotify 同 Retry，每次失败时回调 notify(err, nextWait)
func RetryNotify(operation Operation, b BackOff, notify func(err error, next time.Duration)) error {
	var err error
	for {
		if err = operation(); err == nil {
			return nil
		}
		next := b.NextBackOff()
		if next == Stop {
			return err
		}
		if notify != nil {
			notify(err, next)
		}
		time.Sleep(next)
	}
}
