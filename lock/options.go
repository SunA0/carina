package lock

// LockOption 锁选项
type LockOption struct {
	Key     string
	Timeout int // 秒
	Tries   int
}

// NewLockOption 创建默认锁选项：timeout=10s, tries=3
func NewLockOption(key string) *LockOption {
	return &LockOption{
		Key:     key,
		Timeout: 10,
		Tries:   3,
	}
}

// SetTimeout 设置锁超时时间（秒）
func (o *LockOption) SetTimeout(seconds int) *LockOption {
	o.Timeout = seconds
	return o
}

// SetTries 设置获取锁的重试次数
func (o *LockOption) SetTries(n int) *LockOption {
	o.Tries = n
	return o
}
