package vanilla

import (
	"testing"
)

// TestDoPaginate 纯函数分页计算
func TestDoPaginate(t *testing.T) {
	// 100 条，每页 10 条，第 1 页
	result := doPaginate(100, 1, 10)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.MaxPage != 10 {
		t.Fatalf("expected 10 pages, got %d", result.MaxPage)
	}
	if result.TotalCount != 100 {
		t.Fatalf("expected 100 items, got %d", result.TotalCount)
	}

	// 0 条数据（vanilla 行为：空数据也返回 1 页）
	empty := doPaginate(0, 1, 10)
	if empty == nil {
		t.Fatal("empty result should not be nil")
	}
	if empty.MaxPage != 1 {
		t.Fatalf("expected 1 page for empty (vanilla behavior), got %d", empty.MaxPage)
	}
}

// TestGetTotalPageCount 总页数计算边界
func TestGetTotalPageCount(t *testing.T) {
	cases := []struct {
		total   int64
		perPage int
		want    int
	}{
		{0, 10, 1}, // vanilla 行为：空数据返回 1 页
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{100, 10, 10},
		{101, 10, 11},
	}
	for _, c := range cases {
		got := getTotalPageCount(c.total, c.perPage)
		if got != c.want {
			t.Fatalf("getTotalPageCount(%d,%d)=%d want %d", c.total, c.perPage, got, c.want)
		}
	}
}

// TestParseParamHelpers 参数解析小函数
func TestParseParamHelpers(t *testing.T) {
	if v, err := parseParamInt("42"); err != nil || v != 42 {
		t.Fatalf("parseParamInt(42)=%v,%v", v, err)
	}
	if _, err := parseParamInt("abc"); err == nil {
		t.Fatal("parseParamInt(abc) should fail")
	}
	if v, err := parseParamFloat("3.14"); err != nil || v != 3.14 {
		t.Fatalf("parseParamFloat(3.14)=%v,%v", v, err)
	}
	if !isValidBool("true") || !isValidBool("false") || isValidBool("xyz") {
		t.Fatal("isValidBool boundary failed")
	}
}
