package vanilla

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type responseProbeKey struct{}

type responseProbe struct {
	tx          mockTxOptions
	mode        string
	committed   bool
	afterCalls  int
	finishCalls int
	order       []string
}

type bufferedResource struct{ RestResource }

func (*bufferedResource) Resource() string   { return "buffered.response" }
func (*bufferedResource) GetAlias() []string { return []string{"buffered-alias"} }

func (r *bufferedResource) probe() *responseProbe {
	return r.Ctx.Request.Context().Value(responseProbeKey{}).(*responseProbe)
}

func (r *bufferedResource) PrepareOrm(*gorm.DB) {
	if r.probe().mode == "orm-panic" {
		r.ReturnJSON(MakeResponse(Map{"early": true}))
		panic(NewBusinessError("test:orm_panic", "ORM hook failed"))
	}
}

func (r *bufferedResource) Prepare() {
	if r.probe().mode == "prepare-panic" {
		r.ReturnJSON(MakeResponse(Map{"early": true}))
		panic(NewBusinessError("test:prepare_panic", "prepare failed"))
	}
}

func (r *bufferedResource) Post() {
	p := r.probe()
	r.Ctx.Header("X-Resource", "pending")
	r.Ctx.SetCookie("session", "pending", 60, "/", "", false, true)
	response := MakeResponse(Map{"id": r.Ctx.Query("id")})
	switch p.mode {
	case "raw":
		r.Ctx.JSON(http.StatusCreated, response)
	case "write-string":
		r.Ctx.Header("Content-Type", "application/json")
		_, _ = r.Ctx.Writer.WriteString(`{"code":200,"data":{"id":"raw"}}`)
	case "encode-error":
		r.Ctx.JSON(http.StatusOK, Map{"unsupported": make(chan int)})
	case "oversize":
		_, _ = r.Ctx.Writer.Write([]byte(strings.Repeat("x", 65)))
	case "flush":
		r.Ctx.Writer.Flush()
	case "hijack":
		_, _, _ = r.Ctx.Writer.Hijack()
	case "panic":
		r.ReturnJSON(response)
		panic(NewBusinessError("test:panic", "business failed"))
	case "gin-error":
		r.ReturnJSON(response)
		_ = r.Ctx.Error(errors.New("test: business error"))
	case "return-error":
		r.ReturnError(422, "test:declined", "request declined")
	case "empty":
		return
	default:
		r.ReturnJSONWithCallback(response, func() {
			p.afterCalls++
			p.order = append(p.order, "after")
			if !p.committed {
				panic("test: callback ran before successful commit")
			}
			if p.mode == "after-panic" {
				panic("test: after commit failed")
			}
			if p.mode == "after-write" {
				r.Ctx.Header("X-After", "must-not-leak")
				r.ReturnJSON(MakeResponse(Map{"second": true}))
			}
		})
	}
}

func (r *bufferedResource) Get() { r.Post() }

func (r *bufferedResource) Finish() {
	p := r.probe()
	p.finishCalls++
	p.order = append(p.order, "finish")
	if p.mode == "finish-panic" {
		panic("test: finish failed")
	}
}

func bufferedRouter(t *testing.T, opts HandlerOptions) *gin.Engine {
	t.Helper()
	ensureMockDB(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RecoverPanic())
	engine.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "https://example.com")
	})
	RouterWithOptions(engine, &bufferedResource{}, opts)
	return engine
}

func probeRequest(method, path string, p *responseProbe) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), responseProbeKey{}, p)
	ctx = context.WithValue(ctx, mockTxOptionsKey{}, &p.tx)
	return req.WithContext(ctx)
}

func decodeSingleResponse(t *testing.T, w *httptest.ResponseRecorder) Response {
	t.Helper()
	if !json.Valid(w.Body.Bytes()) {
		t.Fatalf("response must be one JSON document: %s", w.Body.String())
	}
	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestBufferedTransactionResponse(t *testing.T) {
	for _, tt := range []struct {
		mode       string
		commitErr  error
		wantErr    string
		wantCommit int64
		wantAfter  int
	}{
		{mode: "success", wantCommit: 1, wantAfter: 1},
		{mode: "commit-error", commitErr: errors.New("test: commit failed"), wantErr: "rest:tx_commit_failed", wantCommit: 1},
		{mode: "raw", wantCommit: 1},
		{mode: "write-string", wantCommit: 1},
		{mode: "encode-error", wantErr: "rest:response_failed"},
		{mode: "oversize", wantErr: "rest:response_failed"},
		{mode: "flush", wantErr: "rest:tx_response_stream_unsupported"},
		{mode: "hijack", wantErr: "rest:response_failed"},
		{mode: "panic", wantErr: "test:panic"},
		{mode: "orm-panic", wantErr: "test:orm_panic"},
		{mode: "prepare-panic", wantErr: "test:prepare_panic"},
		{mode: "gin-error", wantErr: "rest:response_failed"},
		{mode: "after-panic", wantCommit: 1, wantAfter: 1},
		{mode: "after-write", wantCommit: 1, wantAfter: 1},
		{mode: "finish-panic", wantCommit: 1, wantAfter: 1},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			limit := 4096
			if tt.mode == "oversize" {
				limit = 64
			}
			engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true, MaxResponseBytes: limit})
			stats := ensureMockDB(t)
			commits, rollbacks := atomic.LoadInt64(&stats.commits), atomic.LoadInt64(&stats.rollbacks)
			w := httptest.NewRecorder()
			p := &responseProbe{mode: tt.mode}
			p.tx.commitErr = tt.commitErr
			p.tx.onCommit = func() {
				if w.Body.Len() != 0 || w.Header().Get("X-Resource") != "" || w.Header().Get("Set-Cookie") != "" {
					t.Error("response escaped before Commit completed")
				}
				p.order = append(p.order, "commit")
				p.committed = p.tx.commitErr == nil
			}
			engine.ServeHTTP(w, probeRequest(http.MethodPost, "/buffered/response/?id=42", p))
			response := decodeSingleResponse(t, w)
			if response.ErrCode != tt.wantErr {
				t.Fatalf("errCode=%q want %q, body=%s", response.ErrCode, tt.wantErr, w.Body.String())
			}
			if got := atomic.LoadInt64(&stats.commits) - commits; got != tt.wantCommit {
				t.Errorf("commits=%d want %d", got, tt.wantCommit)
			}
			wantRollback := int64(0)
			if tt.wantCommit == 0 {
				wantRollback = 1
			}
			if got := atomic.LoadInt64(&stats.rollbacks) - rollbacks; got != wantRollback {
				t.Errorf("rollbacks=%d want %d", got, wantRollback)
			}
			if p.afterCalls != tt.wantAfter {
				t.Errorf("afterCalls=%d want %d", p.afterCalls, tt.wantAfter)
			}
			if tt.wantErr != "" {
				if p.finishCalls != 0 || response.Code != 500 || w.Header().Get("X-Resource") != "" || len(w.Result().Cookies()) != 0 {
					t.Errorf("failed request leaked success headers or Finish: response=%+v headers=%v", response, w.Header())
				}
			} else {
				if p.finishCalls != 1 || response.Code != 200 || w.Header().Get("X-Resource") != "pending" || len(w.Result().Cookies()) != 1 {
					t.Errorf("successful response or Finish lost: response=%+v headers=%v", response, w.Header())
				}
				wantStatus := http.StatusOK
				if tt.mode == "raw" {
					wantStatus = http.StatusCreated
				}
				if w.Code != wantStatus || w.Header().Get("X-After") != "" {
					t.Errorf("unexpected status or after-commit headers: status=%d headers=%v", w.Code, w.Header())
				}
				wantOrder := "commit,finish"
				if tt.wantAfter == 1 {
					wantOrder = "commit,after,finish"
				}
				if strings.Join(p.order, ",") != wantOrder {
					t.Errorf("lifecycle order=%v want %s", p.order, wantOrder)
				}
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
				t.Error("upstream CORS header lost")
			}
		})
	}
}

func TestBufferedResponseEmptyAndAlreadyWritten(t *testing.T) {
	for _, alreadyWritten := range []bool{false, true} {
		t.Run(fmt.Sprint(alreadyWritten), func(t *testing.T) {
			ensureMockDB(t)
			engine := gin.New()
			engine.Use(RecoverPanic())
			if alreadyWritten {
				engine.Use(func(c *gin.Context) { c.JSON(http.StatusAccepted, MakeResponse(Map{"upstream": true})) })
			}
			engine.POST("/", CreateHandlerWithOptions(&bufferedResource{}, HandlerOptions{BufferResponseUntilCommit: true}))
			stats := ensureMockDB(t)
			begins, commits := atomic.LoadInt64(&stats.begins), atomic.LoadInt64(&stats.commits)
			p := &responseProbe{mode: "empty"}
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, probeRequest(http.MethodPost, "/", p))
			if alreadyWritten {
				response := decodeSingleResponse(t, w)
				if w.Code != http.StatusAccepted || response.Code != 200 || p.finishCalls != 0 || atomic.LoadInt64(&stats.begins) != begins {
					t.Fatal("handler must not begin a transaction or append a response after upstream output")
				}
			} else if w.Code != http.StatusOK || w.Body.Len() != 0 || p.finishCalls != 1 || atomic.LoadInt64(&stats.commits) != commits+1 {
				t.Fatal("empty response must be committed and published normally")
			}
		})
	}
}

type failingResponseWriter struct {
	header http.Header
	writes int
}

func (w *failingResponseWriter) Header() http.Header { return w.header }
func (*failingResponseWriter) WriteHeader(int)       {}
func (w *failingResponseWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, errors.New("test: connection closed")
}

func TestBufferedResponseNetworkFailure(t *testing.T) {
	engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true})
	stats := ensureMockDB(t)
	commits, rollbacks := atomic.LoadInt64(&stats.commits), atomic.LoadInt64(&stats.rollbacks)
	w := &failingResponseWriter{header: make(http.Header)}
	p := &responseProbe{}
	p.tx.onCommit = func() { p.committed = true }
	engine.ServeHTTP(w, probeRequest(http.MethodPost, "/buffered/response/", p))
	if w.writes != 1 || p.afterCalls != 1 || p.finishCalls != 1 || atomic.LoadInt64(&stats.commits) != commits+1 || atomic.LoadInt64(&stats.rollbacks) != rollbacks {
		t.Fatal("network error must not cause another response, commit, rollback or callback")
	}
}

func TestTransactionResponseHeaderSnapshotAndLimit(t *testing.T) {
	for _, limit := range []int{0, -1, 4} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		buffer := newTransactionResponse(c.Writer, limit)
		if limit <= 0 && buffer.limit != 1<<20 {
			t.Fatal("zero and negative limits must use the bounded default")
		}
		buffer.Header().Set("X-Snapshot", "before")
		buffer.WriteHeader(http.StatusCreated)
		if _, err := buffer.WriteString("test"); err != nil {
			t.Fatal(err)
		}
		buffer.Header().Set("X-Snapshot", "after")
		buffer.WriteHeader(http.StatusBadRequest)
		if w.Body.Len() != 0 || buffer.Status() != http.StatusCreated || !buffer.Written() || buffer.Size() != 4 {
			t.Fatal("buffer must emulate writer state without publishing")
		}
		if limit == 4 {
			if _, err := buffer.WriteString("x"); err == nil || buffer.body.Len() != 4 {
				t.Fatal("overflow must fail without adding bytes")
			}
			if _, err := buffer.Write(nil); err == nil {
				t.Fatal("overflow must remain an error even if the caller ignores it")
			}
			continue
		}
		buffer.seal()
		if _, err := buffer.WriteString("late"); err == nil {
			t.Fatal("sealed response must reject additional output")
		}
		if err := buffer.publish(); err != nil {
			t.Fatal(err)
		}
		if w.Header().Get("X-Snapshot") != "before" || w.Code != http.StatusCreated || w.Body.String() != "test" {
			t.Fatal("first write must freeze HTTP headers and status")
		}
	}
}

func TestBufferedResponseBeforeCommitPanic(t *testing.T) {
	previous := GetBeforeCommitCallback()
	t.Cleanup(func() { SetBeforeCommitCallback(previous) })
	SetBeforeCommitCallback(func(context.Context) { panic(NewBusinessError("test:before_commit", "failed")) })
	engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true})
	stats := ensureMockDB(t)
	commits, rollbacks := atomic.LoadInt64(&stats.commits), atomic.LoadInt64(&stats.rollbacks)
	w := httptest.NewRecorder()
	p := &responseProbe{}
	engine.ServeHTTP(w, probeRequest(http.MethodPost, "/buffered/response/", p))
	response := decodeSingleResponse(t, w)
	if response.ErrCode != "test:before_commit" || p.afterCalls != 0 || p.finishCalls != 0 {
		t.Fatalf("response=%+v probe=%+v", response, p)
	}
	if atomic.LoadInt64(&stats.commits) != commits || atomic.LoadInt64(&stats.rollbacks) != rollbacks+1 {
		t.Fatal("before commit panic must roll back exactly once without committing")
	}
}

func TestBufferedResponseRoutesAndZeroOptions(t *testing.T) {
	for _, buffered := range []bool{false, true} {
		for _, path := range []string{"/buffered/response/", "/buffered/api/response/", "/buffered-alias/"} {
			t.Run(fmt.Sprintf("%t%s", buffered, path), func(t *testing.T) {
				engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: buffered})
				w := httptest.NewRecorder()
				p := &responseProbe{}
				p.tx.onCommit = func() {
					if (w.Body.Len() == 0) != buffered {
						t.Error("response timing does not match Options")
					}
					p.committed = true
				}
				engine.ServeHTTP(w, probeRequest(http.MethodPost, path, p))
				if response := decodeSingleResponse(t, w); response.Code != 200 {
					t.Fatalf("response=%+v", response)
				}
			})
		}
	}
}

func TestBufferedResponseBeginFailure(t *testing.T) {
	engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true})
	w := httptest.NewRecorder()
	p := &responseProbe{tx: mockTxOptions{beginErr: errors.New("test: begin failed")}}
	engine.ServeHTTP(w, probeRequest(http.MethodPost, "/buffered/response/", p))
	if response := decodeSingleResponse(t, w); response.ErrCode != "rest:tx_begin_failed" || p.afterCalls != 0 || p.finishCalls != 0 {
		t.Fatalf("response=%+v probe=%+v", response, p)
	}
}

func TestBufferedResponseWithoutTransaction(t *testing.T) {
	engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true, MaxResponseBytes: 1})
	stats := ensureMockDB(t)
	begins := atomic.LoadInt64(&stats.begins)
	w := httptest.NewRecorder()
	p := &responseProbe{mode: "raw"}
	engine.ServeHTTP(w, probeRequest(http.MethodGet, "/buffered/response/", p))
	if response := decodeSingleResponse(t, w); response.Code != 200 || w.Code != 201 || atomic.LoadInt64(&stats.begins) != begins {
		t.Fatalf("nontransactional behavior changed: response=%+v status=%d", response, w.Code)
	}
}

func TestBufferedReturnErrorDoesNotImplyRollback(t *testing.T) {
	engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true})
	stats := ensureMockDB(t)
	commits, rollbacks := atomic.LoadInt64(&stats.commits), atomic.LoadInt64(&stats.rollbacks)
	w := httptest.NewRecorder()
	p := &responseProbe{mode: "return-error"}
	engine.ServeHTTP(w, probeRequest(http.MethodPost, "/buffered/response/", p))
	response := decodeSingleResponse(t, w)
	if response.Code != 422 || response.ErrCode != "test:declined" || atomic.LoadInt64(&stats.commits) != commits+1 || atomic.LoadInt64(&stats.rollbacks) != rollbacks {
		t.Fatal("business response codes must not implicitly change transaction outcome")
	}
}

func TestBufferedResponseConcurrentIsolation(t *testing.T) {
	engine := bufferedRouter(t, HandlerOptions{BufferResponseUntilCommit: true})
	stats := ensureMockDB(t)
	commits, rollbacks := atomic.LoadInt64(&stats.commits), atomic.LoadInt64(&stats.rollbacks)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			p := &responseProbe{}
			if i%2 == 0 {
				p.tx.commitErr = errors.New("test: commit failed")
			}
			p.tx.onCommit = func() { p.committed = p.tx.commitErr == nil }
			engine.ServeHTTP(w, probeRequest(http.MethodPost, fmt.Sprintf("/buffered/response/?id=%d", i), p))
			response := decodeSingleResponse(t, w)
			if i%2 == 0 {
				if response.ErrCode != "rest:tx_commit_failed" || p.afterCalls != 0 {
					t.Errorf("failed request %d: response=%+v", i, response)
				}
			} else if response.Code != 200 || response.Data.(map[string]interface{})["id"] != fmt.Sprint(i) || p.afterCalls != 1 {
				t.Errorf("request %d received another request's response: %+v", i, response)
			}
		}(i)
	}
	wg.Wait()
	if atomic.LoadInt64(&stats.commits)-commits != 40 || atomic.LoadInt64(&stats.rollbacks) != rollbacks {
		t.Fatal("unexpected transaction counts")
	}
}
