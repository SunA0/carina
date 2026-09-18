package auth

import (
	"testing"
)

// TestEncodeDecodeRoundTrip JWT 编解码往返
func TestEncodeDecodeRoundTrip(t *testing.T) {
	Init("test-secret-key")
	token, err := Encode(1001, 2002, 1, 7)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := Decode(token)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if claims.UserId != 1001 {
		t.Fatalf("UserId=%d want 1001", claims.UserId)
	}
	if claims.AuthUserId != 2002 {
		t.Fatalf("AuthUserId=%d want 2002", claims.AuthUserId)
	}
}

// TestParseUserId 解析用户 ID
func TestParseUserId(t *testing.T) {
	Init("test-secret-key")
	token, err := Encode(3003, 4004, 1, 7)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	userId, authUserId, err := ParseUserId(token)
	if err != nil {
		t.Fatalf("ParseUserId: %v", err)
	}
	if userId != 3003 || authUserId != 4004 {
		t.Fatalf("ParseUserId=%d,%d want 3003,4004", userId, authUserId)
	}
}

// TestDecodeInvalid 非法 token 报错
func TestDecodeInvalid(t *testing.T) {
	Init("test-secret-key")
	if _, err := Decode("not-a-token"); err == nil {
		t.Fatal("Decode(garbage) should fail")
	}

	// 不同密钥签名应失败
	Init("secret-A")
	token, _ := Encode(1, 2, 1, 1)
	Init("secret-B")
	if _, err := Decode(token); err == nil {
		t.Fatal("Decode with wrong secret should fail")
	}
}
