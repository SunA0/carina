package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/suna0/carina/metrics"
	"github.com/suna0/carina/vanilla"
)

// RestRequest WebSocket REST 请求
type RestRequest struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Params string `json:"params"`
	Rid    string `json:"rid"`
}

// WsResponse WebSocket 响应
type WsResponse struct {
	*vanilla.Response
	Rid string `json:"rid"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	writeWait  = 10 * time.Second
	readWait   = 60 * time.Second
	pongWait   = readWait
	pingPeriod = (pongWait * 8) / 10
)

// RestProxyHandler 创建 WebSocket REST 代理 handler。
// 挂载到 Gin engine 上，接收 WebSocket 连接并代理 REST 请求。
func RestProxyHandler(engine *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			if metrics.RestwsErrorCounter != nil {
				metrics.RestwsErrorCounter.WithLabelValues("upgrade").Inc()
			}
			log.Printf("[ws] upgrade error: %v", err)
			return
		}
		if metrics.RestwsGauge != nil {
			metrics.RestwsGauge.Inc()
		}

		respChan := make(chan WsResponse, 256)
		stopRespCh := make(chan struct{})
		closeCh := make(chan struct{})

		defer func() {
			close(closeCh)
			ws.Close()
			if metrics.RestwsGauge != nil {
				metrics.RestwsGauge.Dec()
			}
		}()

		go wsWriter(ws, respChan, stopRespCh, closeCh)
		wsReader(ws, engine, c, respChan, stopRespCh)
	}
}

func wsReader(ws *websocket.Conn, engine *gin.Engine, c *gin.Context, respChan chan<- WsResponse, stopRespCh chan struct{}) {
	for {
		req := new(RestRequest)
		ws.SetReadDeadline(time.Now().Add(readWait))
		err := ws.ReadJSON(req)
		if err != nil {
			if metrics.RestwsErrorCounter != nil {
				metrics.RestwsErrorCounter.WithLabelValues("reader").Inc()
			}
			break
		}
		go wsHandle(engine, c, req, respChan, stopRespCh)
	}
}

func wsHandle(engine *gin.Engine, c *gin.Context, req *RestRequest, respChan chan<- WsResponse, stopRespCh chan struct{}) {
	defer func() {
		if err := recover(); err != nil {
			if metrics.RestwsErrorCounter != nil {
				metrics.RestwsErrorCounter.WithLabelValues("handle").Inc()
			}
			respChan <- WsResponse{
				Response: vanilla.MakeErrorResponse(541, "restws:exception", fmt.Sprintf("%v", err)),
				Rid:      req.Rid,
			}
		}
	}()

	resp := handleWSRequest(engine, c, req)
	select {
	case <-stopRespCh:
		return
	case respChan <- resp:
	}
}

func wsWriter(ws *websocket.Conn, respChan <-chan WsResponse, stopRespCh chan struct{}, closeCh chan struct{}) {
	defer func() {
		if err := recover(); err != nil {
			if metrics.RestwsErrorCounter != nil {
				metrics.RestwsErrorCounter.WithLabelValues("writer_panic").Inc()
			}
		}
	}()
	defer func() {
		close(stopRespCh)
		ws.Close()
	}()

	for {
		select {
		case <-closeCh:
			return
		case resp := <-respChan:
			content, err := json.Marshal(resp)
			if err != nil {
				return
			}
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.TextMessage, content); err != nil {
				if metrics.RestwsErrorCounter != nil {
					metrics.RestwsErrorCounter.WithLabelValues("writer").Inc()
				}
				return
			}
		}
	}
}

func handleWSRequest(engine *gin.Engine, rawCtx *gin.Context, req *RestRequest) WsResponse {
	req.Method = strings.ToUpper(req.Method)
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}
	if !strings.HasSuffix(req.Path, "/") {
		req.Path += "/"
	}

	// 创建 mock 的 http.Request
	mockReq := &http.Request{
		Method: req.Method,
		URL:    &url.URL{Scheme: "http", Host: "localhost", Path: req.Path},
		Header: rawCtx.Request.Header.Clone(),
	}

	// 解析参数
	params := url.Values{}
	var data map[string]interface{}
	json.Unmarshal([]byte(req.Params), &data)
	for k, v := range data {
		params.Set(k, fmt.Sprintf("%v", v))
	}
	mockReq.Form = params

	// 创建 recorder 捕获响应
	recorder := &responseRecorder{
		headers: make(http.Header),
		body:    &strings.Builder{},
		status:  200,
	}

	// 查找并执行 handler
	engine.ServeHTTP(recorder, mockReq)

	// 解析响应
	var resp vanilla.Response
	body := recorder.body.String()
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return WsResponse{
			Response: vanilla.MakeErrorResponse(541, "restws:parse_error", "failed to parse response"),
			Rid:      req.Rid,
		}
	}

	return WsResponse{
		Response: &resp,
		Rid:      req.Rid,
	}
}

// responseRecorder 实现 http.ResponseWriter 接口，捕获响应
type responseRecorder struct {
	headers http.Header
	body    *strings.Builder
	status  int
	mu      sync.Mutex
}

func (r *responseRecorder) Header() http.Header    { return r.headers }
func (r *responseRecorder) WriteHeader(status int) { r.status = status }
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.WriteString(string(b))
}
