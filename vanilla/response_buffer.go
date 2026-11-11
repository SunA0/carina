package vanilla

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

const defaultMaxResponseBytes = 1 << 20

// transactionResponse 暂存单个请求的响应，不向底层 writer 提前写入。
// 不嵌入原 writer，防止 Flush、Hijack 等方法绕过缓冲。
type transactionResponse struct {
	target  gin.ResponseWriter
	header  http.Header
	sent    http.Header
	body    bytes.Buffer
	status  int
	written bool
	sealed  bool
	limit   int
	err     error
}

func newTransactionResponse(target gin.ResponseWriter, limit int) *transactionResponse {
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	return &transactionResponse{
		target: target, header: target.Header().Clone(), status: target.Status(), limit: limit,
	}
}

func (w *transactionResponse) Header() http.Header { return w.header }
func (w *transactionResponse) Status() int         { return w.status }
func (w *transactionResponse) Written() bool       { return w.written }
func (w *transactionResponse) Size() int {
	if !w.written {
		return -1
	}
	return w.body.Len()
}

func (w *transactionResponse) WriteHeader(code int) {
	if w.written || w.sealed || code <= 0 {
		return
	}
	if code < 100 || code > 999 {
		panic(fmt.Sprintf("vanilla: invalid HTTP status code %d", code))
	}
	w.status = code
}

func (w *transactionResponse) WriteHeaderNow() {
	if !w.written && !w.sealed && w.err == nil {
		w.written = true
		w.sent = w.header.Clone()
	}
}

func (w *transactionResponse) Write(data []byte) (int, error) {
	if w.sealed {
		return 0, fmt.Errorf("vanilla: response already finalized")
	}
	if w.err != nil {
		return 0, w.err
	}
	if len(data) > w.limit-w.body.Len() {
		w.err = fmt.Errorf("vanilla: transaction response exceeds %d bytes", w.limit)
		return 0, w.err
	}
	w.WriteHeaderNow()
	return w.body.Write(data)
}

func (w *transactionResponse) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func (w *transactionResponse) Flush() {
	w.err = fmt.Errorf("vanilla: streaming is unavailable with transaction response buffering")
	panic(NewBusinessError("rest:tx_response_stream_unsupported", w.err.Error()))
}

func (w *transactionResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.err = fmt.Errorf("vanilla: hijacking is unavailable with transaction response buffering")
	return nil, nil, w.err
}

func (w *transactionResponse) CloseNotify() <-chan bool { return w.target.CloseNotify() }
func (w *transactionResponse) Pusher() http.Pusher      { return nil }

// seal 冻结提交时的响应，提交后钩子不得再改变已确定的响应。
func (w *transactionResponse) seal() {
	if !w.written {
		w.sent = w.header.Clone()
	}
	w.sealed = true
}

func (w *transactionResponse) publish() error {
	header := w.target.Header()
	clear(header)
	for key, values := range w.sent {
		header[key] = append([]string(nil), values...)
	}
	w.target.WriteHeader(w.status)
	if w.body.Len() == 0 {
		w.target.WriteHeaderNow()
		return nil
	}
	_, err := w.target.Write(w.body.Bytes())
	return err
}

var _ gin.ResponseWriter = (*transactionResponse)(nil)
