package vanilla

import (
	"github.com/suna0/carina/machine"
)

// Map 通用 map 类型
type Map = map[string]interface{}

// FillOption 填充选项
type FillOption = map[string]bool

// Response 统一 API 响应结构
type Response struct {
	Code        int32       `json:"code"`
	Data        interface{} `json:"data"`
	ErrCode     string      `json:"errCode"`
	ErrMsg      string      `json:"errMsg"`
	InnerErrMsg string      `json:"innerErrMsg"`
	MachineInfo Map         `json:"_pod"`
}

// MakeResponse 成功响应
func MakeResponse(data interface{}) *Response {
	return &Response{
		Code:        200,
		Data:        data,
		MachineInfo: machine.Info(),
	}
}

// MakeResponseWithCode 带状态码的成功响应
func MakeResponseWithCode(code int32, data interface{}) *Response {
	return &Response{
		Code:        code,
		Data:        data,
		MachineInfo: machine.Info(),
	}
}

// MakeErrorResponse 错误响应
func MakeErrorResponse(code int32, errCode string, errMsg string, innerErrMsgs ...string) *Response {
	innerErrMsg := ""
	if len(innerErrMsgs) > 0 {
		innerErrMsg = innerErrMsgs[0]
	}
	return &Response{
		Code:        code,
		ErrCode:     errCode,
		ErrMsg:      errMsg,
		InnerErrMsg: innerErrMsg,
		MachineInfo: machine.Info(),
	}
}
