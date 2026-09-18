package vanilla

import "fmt"

// ErrorType 错误类型
type ErrorType int

const (
	// ErrorTypeBusiness 业务错误（正常流程，不推送 Sentry）
	ErrorTypeBusiness ErrorType = 0
	// ErrorTypeSystem 系统错误（需要关注，推送 Sentry）
	ErrorTypeSystem ErrorType = 1
)

// BusinessError 业务错误，通过 panic 抛出，由 RecoverPanic 统一处理。
type BusinessError struct {
	Type             ErrorType
	ErrCode          string
	ErrMsg           string
	needPushToSentry bool
}

// Error 实现 error 接口
func (e *BusinessError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrCode, e.ErrMsg)
}

// IsPanicError 是否为系统级错误
func (e *BusinessError) IsPanicError() bool {
	return e.Type == ErrorTypeSystem
}

// NoPush 设置不推送 Sentry
func (e *BusinessError) NoPush() *BusinessError {
	e.needPushToSentry = false
	return e
}

// IsNeedPush 是否需要推送 Sentry
func (e *BusinessError) IsNeedPush() bool {
	return e.needPushToSentry || e.IsPanicError()
}

// NewBusinessError 创建业务错误
func NewBusinessError(code string, msg string) *BusinessError {
	return &BusinessError{
		Type:             ErrorTypeBusiness,
		ErrCode:          code,
		ErrMsg:           msg,
		needPushToSentry: true,
	}
}

// NewBusinessErrorFromError 从 error 创建 BusinessError
func NewBusinessErrorFromError(err error) *BusinessError {
	if be, ok := err.(*BusinessError); ok {
		return be
	}
	return &BusinessError{
		Type:             ErrorTypeBusiness,
		ErrCode:          err.Error(),
		ErrMsg:           err.Error(),
		needPushToSentry: true,
	}
}

// NewSystemError 创建系统错误
func NewSystemError(code string, msg string) *BusinessError {
	return &BusinessError{
		Type:             ErrorTypeSystem,
		ErrCode:          code,
		ErrMsg:           msg,
		needPushToSentry: true,
	}
}
