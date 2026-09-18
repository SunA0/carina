package vanilla

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/suna0/carina/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// ---- mock database/sql 驱动：统计事务 Begin/Commit/Rollback，不依赖真实 DB ----

type mockStats struct {
	begins    int64
	commits   int64
	rollbacks int64
}

type mockDriver struct {
	stats *mockStats
}

func (d *mockDriver) Open(string) (driver.Conn, error) { return &mockConn{d}, nil }

// mockConnector 免去 sql.Register，直接交给 sql.OpenDB
type mockConnector struct{ d *mockDriver }

func (c *mockConnector) Connect(context.Context) (driver.Conn, error) { return &mockConn{c.d}, nil }
func (c *mockConnector) Driver() driver.Driver                     { return c.d }
func (c *mockConnector) Close() error                              { return nil }

type mockConn struct{ d *mockDriver }

func (c *mockConn) Prepare(string) (driver.Stmt, error) { return &mockStmt{}, nil }
func (c *mockConn) Close() error                        { return nil }
func (c *mockConn) Begin() (driver.Tx, error) {
	atomic.AddInt64(&c.d.stats.begins, 1)
	return &mockTx{d: c.d}, nil
}

type mockTx struct{ d *mockDriver }

func (t *mockTx) Commit() error {
	atomic.AddInt64(&t.d.stats.commits, 1)
	return nil
}
func (t *mockTx) Rollback() error {
	atomic.AddInt64(&t.d.stats.rollbacks, 1)
	return nil
}

type mockStmt struct{}

func (s *mockStmt) Close() error  { return nil }
func (s *mockStmt) NumInput() int { return -1 }
func (s *mockStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.ResultNoRows, nil
}
func (s *mockStmt) Query([]driver.Value) (driver.Rows, error) {
	return &mockRows{}, nil
}

type mockRows struct{}

func (r *mockRows) Columns() []string { return nil }
func (r *mockRows) Close() error      { return nil }
func (r *mockRows) Next([]driver.Value) error {
	return io.EOF
}

// mockDialector 复用 db/filter_test.go 的 fake 驱动思路，
// Initialize 注入基于 mock 驱动的 *sql.DB，支持真实 Begin/Commit/Rollback 流程。
type mockDialector struct {
	stats *mockStats
}

func (mockDialector) Name() string { return "mock" }
func (m mockDialector) Initialize(gdb *gorm.DB) error {
	gdb.ConnPool = sql.OpenDB(&mockConnector{d: &mockDriver{stats: m.stats}})
	return nil
}
func (mockDialector) Migrator(gdb *gorm.DB) gorm.Migrator  { return nil }
func (mockDialector) DataTypeOf(*schema.Field) string      { return "" }
func (mockDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return clause.Expr{SQL: "DEFAULT"}
}
func (mockDialector) BindVarTo(w clause.Writer, stmt *gorm.Statement, v interface{}) {
	w.WriteByte('?')
}
func (mockDialector) QuoteTo(w clause.Writer, s string) {
	w.WriteByte('`')
	w.WriteString(s)
	w.WriteByte('`')
}
func (mockDialector) Explain(sql string, vars ...interface{}) string { return sql }

var (
	mockDBOnce  sync.Once
	mockDBStats = &mockStats{}
)

// ensureMockDB 初始化全局 mock DB（db.Init 幂等，全测试二进制共享）
func ensureMockDB(t *testing.T) *mockStats {
	t.Helper()
	mockDBOnce.Do(func() {
		if err := db.Init(mockDialector{stats: mockDBStats}, nil); err != nil {
			t.Errorf("db.Init: %v", err)
		}
	})
	return mockDBStats
}

// ---- 开启事务的资源（覆盖 handler 的事务分支） ----

// txProfileResource 声明 json 参数并在事务中回显，用于验证解析结果是否存活到方法派发
type txProfileResource struct {
	RestResource
}

func (r *txProfileResource) Resource() string  { return "tx.profile" }
func (r *txProfileResource) DisableTx() bool   { return false } // 所有方法走事务
func (r *txProfileResource) GetLockKey() string { return "" }
func (r *txProfileResource) GetParameters() map[string][]string {
	return map[string][]string{
		"POST": {"?profile:json", "?tags:json-array"},
	}
}
func (r *txProfileResource) Post() {
	r.ReturnJSON(MakeResponse(map[string]interface{}{
		"profile": r.GetJSON("profile"),
		"tags":    r.GetJSONArray("tags"),
	}))
}

var _ RestResourceInterface = (*txProfileResource)(nil)

func setupTxRouter(t *testing.T, r RestResourceInterface) *gin.Engine {
	t.Helper()
	ensureMockDB(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RecoverPanic())
	Router(engine, r)
	return engine
}

// TestTxHandlerPreservesParsedParams 事务开启后，parseParameters 已解析的
// JSON/数组参数不应丢失（handler 在 BeginTx 后二次 InitContext 会重建 map）。
func TestTxHandlerPreservesParsedParams(t *testing.T) {
	engine := setupTxRouter(t, &txProfileResource{})

	body := "profile=" + `{"age":20}` + "&tags=" + `["a","b"]`
	resp := doRequest(t, engine, "POST", "/tx/profile/", body)
	if resp.Code != 200 {
		t.Fatalf("code=%d errCode=%s", resp.Code, resp.ErrCode)
	}
	data := resp.Data.(map[string]interface{})

	profile, ok := data["profile"].(map[string]interface{})
	if !ok || profile["age"].(float64) != 20 {
		t.Errorf("BUG: 事务模式下已解析的 json 参数丢失，profile=%v，want {age:20}", data["profile"])
	}
	if tags, ok := data["tags"].([]interface{}); !ok || len(tags) != 2 {
		t.Errorf("BUG: 事务模式下已解析的 json-array 参数丢失，tags=%v，want 2 个元素", data["tags"])
	}
}

// TestTxHandlerCommitOnSuccess 事务资源请求成功后应提交事务
func TestTxHandlerCommitOnSuccess(t *testing.T) {
	engine := setupTxRouter(t, &txProfileResource{})
	stats := ensureMockDB(t)

	beginBefore := atomic.LoadInt64(&stats.begins)
	commitBefore := atomic.LoadInt64(&stats.commits)
	doRequest(t, engine, "POST", "/tx/profile/", "profile="+`{"a":1}`)
	if after := atomic.LoadInt64(&stats.begins); after != beginBefore+1 {
		t.Fatalf("begins delta=%d want 1", after-beginBefore)
	}
	if delta := atomic.LoadInt64(&stats.commits) - commitBefore; delta != 1 {
		t.Errorf("成功请求后应提交事务：commits delta=%d want 1", delta)
	}
}

// TestTxHandlerBusinessErrorRollback 业务 panic 应回滚事务并返回统一错误
func TestTxHandlerBusinessErrorRollback(t *testing.T) {
	engine := setupTxRouter(t, &txPanicResource{})
	stats := ensureMockDB(t)

	beforeBegin := atomic.LoadInt64(&stats.begins)
	beforeRollback := atomic.LoadInt64(&stats.rollbacks)
	resp := doRequest(t, engine, "POST", "/tx/panic/", "")
	if resp.Code != 500 || resp.ErrCode != "tx:boom" {
		t.Fatalf("resp=%+v want code=500 errCode=tx:boom", resp)
	}
	if delta := atomic.LoadInt64(&stats.rollbacks) - beforeRollback; delta < 1 {
		t.Errorf("panic 后应回滚事务：rollbacks delta=%d（begins 基线=%d）", delta, beforeBegin)
	}
}

type txPanicResource struct {
	RestResource
}

func (r *txPanicResource) Resource() string { return "tx.panic" }
func (r *txPanicResource) DisableTx() bool  { return false }
func (r *txPanicResource) Post() {
	panic(NewBusinessError("tx:boom", "业务异常"))
}

var _ RestResourceInterface = (*txPanicResource)(nil)

// TestTxHandlerMethodNotAllowedNoTxLeak 事务已开启但方法不存在返回 405 时，
// 必须提交/回滚事务，否则事务与连接泄漏。
func TestTxHandlerMethodNotAllowedNoTxLeak(t *testing.T) {
	engine := setupTxRouter(t, &txProfileResource{}) // 该资源未定义 Put
	stats := ensureMockDB(t)

	beginBefore := atomic.LoadInt64(&stats.begins)
	commitBefore := atomic.LoadInt64(&stats.commits)
	rollbackBefore := atomic.LoadInt64(&stats.rollbacks)

	resp := doRequest(t, engine, "PUT", "/tx/profile/", "")
	if resp.Code != 405 {
		t.Fatalf("code=%d want 405", resp.Code)
	}

	begins := atomic.LoadInt64(&stats.begins) - beginBefore
	enders := (atomic.LoadInt64(&stats.commits) - commitBefore) +
		(atomic.LoadInt64(&stats.rollbacks) - rollbackBefore)
	if begins != enders {
		t.Errorf("BUG: 405 路径事务泄漏：begins=%d，commit+rollback=%d（差值 %d 个事务未关闭，占用连接池）",
			begins, enders, begins-enders)
	}
}

// TestTxHandlerConcurrentBalance 并发事务请求：所有 Begin 最终都要有配对的
// Commit/Rollback（配合 -race 检查 handler 并发安全）。
func TestTxHandlerConcurrentBalance(t *testing.T) {
	engine := setupTxRouter(t, &txProfileResource{})
	stats := ensureMockDB(t)

	beginBefore := atomic.LoadInt64(&stats.begins)
	commitBefore := atomic.LoadInt64(&stats.commits)
	rollbackBefore := atomic.LoadInt64(&stats.rollbacks)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := doRequest(t, engine, "POST", "/tx/profile/", `profile={"id":1}`)
			if resp.Code != 200 {
				t.Errorf("code=%d errCode=%s", resp.Code, resp.ErrCode)
			}
		}()
	}
	wg.Wait()

	begins := atomic.LoadInt64(&stats.begins) - beginBefore
	enders := (atomic.LoadInt64(&stats.commits) - commitBefore) +
		(atomic.LoadInt64(&stats.rollbacks) - rollbackBefore)
	if begins != int64(n) {
		t.Fatalf("begins=%d want %d", begins, n)
	}
	if enders != begins {
		t.Errorf("并发请求后事务不守恒：begins=%d enders=%d（存在泄漏）", begins, enders)
	}
}

// TestParseParametersRejectsInvalidJSON json 参数非法时应校验失败
func TestParseParametersRejectsInvalidJSON(t *testing.T) {
	engine := setupTxRouter(t, &txProfileResource{})
	resp := doRequest(t, engine, "POST", "/tx/profile/", "profile=not-json")
	if resp.ErrCode != "rest:missing_argument" {
		t.Errorf("errCode=%s want rest:missing_argument", resp.ErrCode)
	}
	if !strings.Contains(resp.ErrMsg, "profile") {
		t.Errorf("errMsg=%s 应包含参数名", resp.ErrMsg)
	}
}
