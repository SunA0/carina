package snowflake

import (
	"testing"
)

// TestGenerateUnique 同节点连续生成不重复
func TestGenerateUnique(t *testing.T) {
	node, err := NewNode(1)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	seen := make(map[ID]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id := node.Generate()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %v at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}

// TestIDParts 时间/节点/步长可解析
func TestIDParts(t *testing.T) {
	node, err := NewNode(7)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	id := node.Generate()
	if id.Node() != 7 {
		t.Fatalf("node=%d want 7", id.Node())
	}
	if id.Time() < Epoch {
		t.Fatalf("time %d should >= epoch %d", id.Time(), Epoch)
	}
}

// TestBase58RoundTrip base58 编解码往返
func TestBase58RoundTrip(t *testing.T) {
	node, _ := NewNode(1)
	id := node.Generate()
	parsed, err := ParseBase58([]byte(id.Base58()))
	if err != nil {
		t.Fatalf("ParseBase58: %v", err)
	}
	if parsed != id {
		t.Fatalf("roundtrip mismatch: %v vs %v", parsed, id)
	}
}

// TestJSONRoundTrip JSON 编解码往返
func TestJSONRoundTrip(t *testing.T) {
	node, _ := NewNode(1)
	id := node.Generate()
	b, err := id.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var out ID
	if err := out.UnmarshalJSON(b); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if out != id {
		t.Fatalf("json roundtrip mismatch: %v vs %v", out, id)
	}
}

// TestNewNodeBoundary 节点号边界
func TestNewNodeBoundary(t *testing.T) {
	if _, err := NewNode(-1); err == nil {
		t.Fatal("negative node should fail")
	}
	if _, err := NewNode(1 << NodeBits); err == nil {
		t.Fatal("overflow node should fail")
	}
	if _, err := NewNode((1 << NodeBits) - 1); err != nil {
		t.Fatalf("max node should pass: %v", err)
	}
}
