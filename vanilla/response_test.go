package vanilla

import (
	"testing"
)

// TestMakeResponse 统一响应结构
func TestMakeResponse(t *testing.T) {
	resp := MakeResponse(Map{"id": 1})
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if resp.Code != 200 {
		t.Fatalf("expected code 200, got %d", resp.Code)
	}
}

// TestMakeErrorResponse 错误响应结构
func TestMakeErrorResponse(t *testing.T) {
	resp := MakeErrorResponse(500, "test:error", "something wrong")
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if resp.Code != 500 {
		t.Fatalf("expected code 500, got %d", resp.Code)
	}
}

// TestBusinessError 业务错误基本行为
func TestBusinessError(t *testing.T) {
	err := NewBusinessError("corp:not_found", "企业不存在")
	if err.Error() == "" {
		t.Fatal("error message should not be empty")
	}
	if !err.IsNeedPush() {
		t.Fatal("business error should need push by default")
	}
	err.NoPush()
	if err.IsNeedPush() {
		t.Fatal("business error should not need push after NoPush()")
	}
}

// TestNewBusinessErrorFromError 从标准 error 包装
func TestNewBusinessErrorFromError(t *testing.T) {
	base := NewBusinessError("x:y", "msg")
	wrapped := NewBusinessErrorFromError(base)
	if wrapped == nil {
		t.Fatal("wrapped error should not be nil")
	}
}
