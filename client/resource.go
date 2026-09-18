package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/suna0/carina/tracing"
	"go.opentelemetry.io/otel/propagation"
)

// Map 通用 map 类型
type Map = map[string]interface{}

// retryCount 默认重试次数
const defaultRetryCount = 3

var (
	// ServiceName 本服务名称
	ServiceName string
	// ServiceMode 服务模式
	ServiceMode string
	// APIServerHost API 网关地址
	APIServerHost string

	httpDialTimeout      = 5
	httpDialKeepAlive    = 60
	httpIdleConnsPerHost = 1000
	httpMaxIdleConns     = 2000
	httpIdleConnTimeout  = 60
)

// Init 初始化服务间调用客户端
func Init(serviceName, serviceMode, apiServerHost string) {
	ServiceName = serviceName
	ServiceMode = serviceMode
	APIServerHost = apiServerHost
}

// ResourceResponse 服务间调用响应
type ResourceResponse struct {
	RespData map[string]interface{}
}

// IsSuccess 是否成功
func (r *ResourceResponse) IsSuccess() bool {
	code, _ := r.RespData["code"].(float64)
	return int(code) == 200
}

// Data 获取响应数据
func (r *ResourceResponse) Data() interface{} {
	return r.RespData["data"]
}

// Bind 将响应 data 绑定到结构体
func (r *ResourceResponse) Bind(container interface{}) error {
	data, err := json.Marshal(r.RespData["data"])
	if err != nil {
		return err
	}
	return json.Unmarshal(data, container)
}

// Resource 服务间调用客户端。
// 持有 context（含 JWT token + tracing span），发起 HTTP 调用。
type Resource struct {
	Ctx            context.Context
	CustomJWTToken string
	disableRetry   bool
}

// NewResource 创建资源调用客户端
func NewResource(ctx context.Context) *Resource {
	return &Resource{
		Ctx:          ctx,
		disableRetry: false,
	}
}

// DisableRetry 禁用重试
func (r *Resource) DisableRetry() *Resource {
	r.disableRetry = true
	return r
}

// Get 发起 GET 请求
func (r *Resource) Get(service string, resource string, data Map) (*ResourceResponse, error) {
	return r.requestWithRetry("GET", service, resource, data)
}

// Post 发起 POST 请求
func (r *Resource) Post(service string, resource string, data Map) (*ResourceResponse, error) {
	return r.requestWithRetry("POST", service, resource, data)
}

// Put 发起 PUT 请求
func (r *Resource) Put(service string, resource string, data Map) (*ResourceResponse, error) {
	return r.requestWithRetry("PUT", service, resource, data)
}

// Delete 发起 DELETE 请求
func (r *Resource) Delete(service string, resource string, data Map) (*ResourceResponse, error) {
	return r.requestWithRetry("DELETE", service, resource, data)
}

func (r *Resource) requestWithRetry(method, service, resource string, data Map) (*ResourceResponse, error) {
	var resp *ResourceResponse
	var err error
	for i := 0; i < defaultRetryCount; i++ {
		resp, err = r.request(method, service, resource, data)
		if err == nil {
			return resp, nil
		}
		if r.disableRetry {
			break
		}
	}
	return resp, err
}

func (r *Resource) request(method, service, resource string, data Map) (*ResourceResponse, error) {
	// 构建 JWT token
	jwtToken := r.CustomJWTToken
	if jwtToken == "" {
		if v := r.Ctx.Value("jwt"); v != nil {
			jwtToken = v.(string)
		}
	}

	// 构建 URL
	pos := strings.LastIndexByte(resource, '.')
	resourcePath := resource
	if pos >= 0 {
		resourcePath = resource[:pos] + "/" + resource[pos+1:]
	}
	apiURL := fmt.Sprintf("http://%s/%s/%s/", APIServerHost, service, resourcePath)

	// 构建 query params
	params := url.Values{}
	params.Set("_v", "1")
	params.Set("__source_service", ServiceName)
	params.Set("__source_service_v2", fmt.Sprintf("%s-%s", ServiceName, ServiceMode))

	// 创建 HTTP client
	netClient := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   time.Duration(httpDialTimeout) * time.Second,
				KeepAlive: time.Duration(httpDialKeepAlive) * time.Second,
			}).DialContext,
			MaxIdleConnsPerHost:   httpIdleConnsPerHost,
			MaxIdleConns:          httpMaxIdleConns,
			IdleConnTimeout:       time.Duration(httpIdleConnTimeout) * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
		},
	}

	var req *http.Request
	var err error

	if method == "GET" {
		for k, v := range data {
			params.Set(k, fmt.Sprintf("%v", v))
		}
		apiURL += "?" + params.Encode()
		req, err = http.NewRequestWithContext(r.Ctx, "GET", apiURL, nil)
	} else {
		if method == "PUT" {
			params.Set("_method", "put")
		} else if method == "DELETE" {
			params.Set("_method", "delete")
		}
		apiURL += "?" + params.Encode()

		formData := url.Values{}
		for k, v := range data {
			formData.Set(k, fmt.Sprintf("%v", v))
		}
		req, err = http.NewRequestWithContext(r.Ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("client: create request: %w", err)
	}

	// 设置 headers
	req.Header.Set("AUTHORIZATION", jwtToken)

	// 注入 tracing
	tracing.StartSpan(r.Ctx, fmt.Sprintf("client.%s.%s", service, resource))
	// 传播 trace context
	propagator := propagation.TraceContext{}
	propagator.Inject(r.Ctx, propagation.HeaderCarrier(req.Header))

	// 执行请求
	httpResp, err := netClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: do request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("client: read body: %w", err)
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("client: unmarshal response: %w", err)
	}

	return &ResourceResponse{RespData: respData}, nil
}
