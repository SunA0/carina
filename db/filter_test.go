package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// fakeDialector 仅用于 DryRun 模式下生成 SQL，不连接真实数据库。
// 保证框架测试不引入任何具体 DB 驱动。
type fakeDialector struct{}

// fakeConnPool 避免 gorm 对 *sql.DB 发起 Ping
type fakeConnPool struct{}

func (fakeConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, nil
}
func (fakeConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (fakeConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}
func (fakeConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return &sql.Row{}
}

func (fakeDialector) Name() string { return "fake" }
func (fakeDialector) Initialize(db *gorm.DB) error {
	db.ConnPool = fakeConnPool{}
	return nil
}
func (fakeDialector) Migrator(db *gorm.DB) gorm.Migrator { return nil }
func (fakeDialector) DataTypeOf(field *schema.Field) string {
	return ""
}
func (f fakeDialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return clause.Expr{SQL: "DEFAULT"}
}
func (fakeDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}
func (fakeDialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('`')
	writer.WriteString(str)
	writer.WriteByte('`')
}
func (fakeDialector) Explain(sql string, vars ...interface{}) string { return sql }

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(fakeDialector{}, &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("open dry-run db: %v", err)
	}
	return db
}

// whereSQLs 提取 statement 中 WHERE 子句的各表达式 SQL 片段。
// 直接检查 ApplyFilters 拼接的 clause，不依赖真实执行。
func whereSQLs(db *gorm.DB) []string {
	var out []string
	whereClause, ok := db.Statement.Clauses["WHERE"]
	if !ok {
		return out
	}
	where, ok := whereClause.Expression.(clause.Where)
	if !ok {
		return out
	}
	for _, e := range where.Exprs {
		if expr, ok := e.(clause.Expr); ok {
			out = append(out, expr.SQL)
		}
	}
	return out
}

// containsSQL 检查是否存在包含 fragment 的 WHERE 片段
func containsSQL(fragments []string, fragment string) bool {
	for _, f := range fragments {
		if strings.Contains(f, fragment) {
			return true
		}
	}
	return false
}

// TestApplyFiltersEqual 等值过滤
func TestApplyFiltersEqual(t *testing.T) {
	db := ApplyFilters(newDryRunDB(t), map[string]interface{}{"name": "张三"})
	if !containsSQL(whereSQLs(db), "name = ?") {
		t.Fatalf("missing equal clause: %v", whereSQLs(db))
	}
}

// TestApplyFiltersOperators 各操作符 SQL 片段
func TestApplyFiltersOperators(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"__f-age-gt", "age > ?"},
		{"__f-age-gte", "age >= ?"},
		{"__f-age-lt", "age < ?"},
		{"__f-age-lte", "age <= ?"},
		{"__f-name-contain", "name LIKE ?"},
		{"__f-status-in", "status IN ?"},
		{"__f-status-notin", "status NOT IN ?"},
		{"__f-name-equal", "name = ?"},
	}
	for _, c := range cases {
		db := ApplyFilters(newDryRunDB(t), map[string]interface{}{c.key: "x"})
		if !containsSQL(whereSQLs(db), c.want) {
			t.Fatalf("filter %s: clauses %v missing %q", c.key, whereSQLs(db), c.want)
		}
	}
}

// TestApplyFiltersRange 范围过滤
func TestApplyFiltersRange(t *testing.T) {
	db := ApplyFilters(newDryRunDB(t), map[string]interface{}{
		"__f-age-range": []interface{}{10, 20},
	})
	fragments := whereSQLs(db)
	if !containsSQL(fragments, "age >= ?") || !containsSQL(fragments, "age <= ?") {
		t.Fatalf("range clauses invalid: %v", fragments)
	}
}

// TestTxContextRoundTrip 事务 context 存取
func TestTxContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	if TxFromContext(ctx) != nil {
		t.Fatal("empty context should have no tx")
	}
	tx := &gorm.DB{}
	ctx2 := ContextWithTx(ctx, tx)
	if TxFromContext(ctx2) == nil {
		t.Fatal("tx should be retrievable from context")
	}
}
