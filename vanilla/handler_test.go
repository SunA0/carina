package vanilla

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// testResource 内嵌基类的真实业务资源（模拟下游用法）
type testResource struct {
	RestResource
}

func (r *testResource) Resource() string { return "test.item" }

func (r *testResource) DisableTx() bool { return true } // 测试无真实 DB

func (r *testResource) GetParameters() map[string][]string {
	return map[string][]string{
		"GET":  {"?id:int", "?name:string"},
		"POST": {"name:string", "?profile:json", "?tags:json-array"},
	}
}

func (r *testResource) Get() {
	id, _ := r.GetInt("id")
	r.ReturnJSON(MakeResponse(Map{
		"id":      id,
		"name":    r.GetStringDefault("name", "anonymous"),
		"filters": r.GetFilters(),
	}))
}

func (r *testResource) Post() {
	name := r.GetString("name")
	if name == "admin" {
		panic(NewBusinessError("test:name_reserved", "该用户名被保留"))
	}
	r.ReturnJSON(MakeResponse(Map{
		"name":    name,
		"profile": r.GetJSON("profile"),
		"tags":    r.GetStringArray("tags"),
	}))
}

var _ RestResourceInterface = (*testResource)(nil)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RecoverPanic()) // 业务错误流依赖
	Router(engine, &testResource{})
	return engine
}

func doRequest(t *testing.T, engine *gin.Engine, method, target, body string) *Response {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code == http.StatusMethodNotAllowed {
		return &Response{Code: 405}
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v, body: %s", err, w.Body.String())
	}
	return &resp
}

func TestHandlerGET(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "GET", "/test/item/?id=42&name=zhangsan", "")
	if resp.Code != 200 {
		t.Fatalf("code = %d, errCode = %s", resp.Code, resp.ErrCode)
	}
	data := resp.Data.(map[string]interface{})
	if data["id"].(float64) != 42 {
		t.Errorf("id = %v, want 42", data["id"])
	}
	if data["name"] != "zhangsan" {
		t.Errorf("name = %v", data["name"])
	}
}

func TestHandlerGETDefaultParam(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "GET", "/test/item/", "")
	if resp.Code != 200 {
		t.Fatalf("code = %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	if data["name"] != "anonymous" {
		t.Errorf("name = %v, want anonymous", data["name"])
	}
}

func TestHandlerFilters(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "GET", "/test/item/?__f-age-gt=18&__f-id-in=[1,2,3]", "")
	if resp.Code != 200 {
		t.Fatalf("code = %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	filters := data["filters"].(map[string]interface{})
	if filters["__f-age-gt"] != "18" {
		t.Errorf("__f-age-gt = %v", filters["__f-age-gt"])
	}
	arr, ok := filters["__f-id-in"].([]interface{})
	if !ok || len(arr) != 3 {
		t.Errorf("__f-id-in = %v, want 3 elements", filters["__f-id-in"])
	}
}

func TestHandlerPostForm(t *testing.T) {
	engine := setupRouter()
	// POST 表单参数必须能被校验和获取（PostForm 优先语义）
	body := "name=lisi&profile=" + `{"age":20}` + "&tags=" + `["a","b"]`
	resp := doRequest(t, engine, "POST", "/test/item/", body)
	if resp.Code != 200 {
		t.Fatalf("code = %d, errCode = %s", resp.Code, resp.ErrCode)
	}
	data := resp.Data.(map[string]interface{})
	if data["name"] != "lisi" {
		t.Errorf("name = %v", data["name"])
	}
	if data["profile"].(map[string]interface{})["age"].(float64) != 20 {
		t.Errorf("profile.age = %v", data["profile"])
	}
	if tags := data["tags"].([]interface{}); len(tags) != 2 {
		t.Errorf("tags = %v", tags)
	}
}

func TestHandlerMissingRequiredParam(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "POST", "/test/item/", "") // 缺 name
	if resp.ErrCode != "rest:missing_argument" {
		t.Errorf("errCode = %s, want rest:missing_argument", resp.ErrCode)
	}
}

func TestHandlerInvalidIntParam(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "GET", "/test/item/?id=abc", "")
	if resp.ErrCode != "rest:missing_argument" {
		t.Errorf("errCode = %s", resp.ErrCode)
	}
}

func TestHandlerBusinessError(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "POST", "/test/item/", "name=admin")
	// vanilla 语义：BusinessError → code 500（非 BusinessError panic 才是 531）
	if resp.Code != 500 {
		t.Errorf("code = %d, want 500", resp.Code)
	}
	if resp.ErrCode != "test:name_reserved" {
		t.Errorf("errCode = %s", resp.ErrCode)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "DELETE", "/test/item/", "")
	if resp.Code != 405 {
		t.Errorf("code = %d, want 405", resp.Code)
	}
}

func TestHandlerAPIRoute(t *testing.T) {
	engine := setupRouter()
	resp := doRequest(t, engine, "GET", "/test/api/item/?id=7", "")
	if resp.Code != 200 {
		t.Fatalf("code = %d", resp.Code)
	}
	data := resp.Data.(map[string]interface{})
	if data["id"].(float64) != 7 {
		t.Errorf("id = %v, want 7", data["id"])
	}
}

// TestHandlerConcurrentIsolation 并发请求实例隔离：不串参数
func TestHandlerConcurrentIsolation(t *testing.T) {
	engine := setupRouter()
	done := make(chan string, 100)
	for i := 0; i < 100; i++ {
		go func() {
			resp := doRequest(t, engine, "GET", "/test/item/?id=1&name=shared", "")
			data := resp.Data.(map[string]interface{})
			done <- data["name"].(string)
		}()
	}
	for i := 0; i < 100; i++ {
		if name := <-done; name != "shared" {
			t.Errorf("name = %s, want shared (实例串扰)", name)
		}
	}
}

// ptrEmbeddedResource 指针嵌入基类的资源
type ptrEmbeddedResource struct {
	*RestResource
}

func (r *ptrEmbeddedResource) Resource() string { return "test.ptr" }
func (r *ptrEmbeddedResource) DisableTx() bool  { return true }
func (r *ptrEmbeddedResource) Get() {
	r.ReturnJSON(MakeResponse(Map{"ok": true}))
}

var _ RestResourceInterface = (*ptrEmbeddedResource)(nil)

// TestHandlerPtrEmbedded 指针嵌入 nil 基类时框架应自动初始化
func TestHandlerPtrEmbedded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Router(engine, &ptrEmbeddedResource{})
	resp := doRequest(t, engine, "GET", "/test/ptr/", "")
	if resp.Code != 200 {
		t.Fatalf("code = %d, errCode = %s", resp.Code, resp.ErrCode)
	}
}
