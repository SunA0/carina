package cron

import (
	"math"
	"sync"
)

// PipeInterface 管道任务接口
type PipeInterface interface {
	TaskInterface
	AddData(data interface{}) error
	GetData() interface{}
	GetCap() int
	GetConsumerCount() int
	RunConsumer(data interface{}, taskCtx *TaskContext)
	EnableParallel() bool
}

// Pipe 管道任务基类，实现生产者/消费者模式。
// 内嵌此结构体实现 PipeInterface。
type Pipe struct {
	Task
	ch    chan interface{}
	chCap int
	mu    sync.Mutex
}

// NewPipe 创建管道，chCap 为 channel 容量
func NewPipe(chCap int) Pipe {
	return Pipe{
		ch:    make(chan interface{}, chCap),
		chCap: chCap,
	}
}

// GetData 从管道取数据（阻塞）
func (p *Pipe) GetData() interface{} {
	return <-p.ch
}

// AddData 向管道添加数据（非阻塞，满时返回错误）
func (p *Pipe) AddData(data interface{}) error {
	select {
	case p.ch <- data:
		return nil
	default:
		return ErrChannelFull
	}
}

// GetCap 返回管道容量
func (p *Pipe) GetCap() int {
	return p.chCap
}

// GetConsumerCount 消费者数量，默认为容量的十分之一
func (p *Pipe) GetConsumerCount() int {
	return int(math.Ceil(float64(p.GetCap()) / 10))
}

// EnableParallel 是否并行消费，默认 true
func (p *Pipe) EnableParallel() bool {
	return true
}
