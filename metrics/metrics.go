package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	once sync.Once

	// EndpointCounter 按资源+方法统计请求数
	EndpointCounter *prometheus.CounterVec
	// EndpointDuration 按资源+方法统计请求耗时
	EndpointDuration *prometheus.HistogramVec
	// EndpointCallByService 按调用来源服务统计
	EndpointCallByService *prometheus.CounterVec
	// RemoteEndpointCallCounter 服务间调用统计
	RemoteEndpointCallCounter *prometheus.CounterVec
	// PanicCounter panic 计数
	PanicCounter prometheus.Counter
	// BusinessErrorCounter 业务错误计数
	BusinessErrorCounter prometheus.Counter
	// ResourceRetryCounter 资源重试计数
	ResourceRetryCounter prometheus.Counter
	// RestwsGauge WebSocket 连接数
	RestwsGauge prometheus.Gauge
	// RestwsErrorCounter WebSocket 错误计数
	RestwsErrorCounter *prometheus.CounterVec
	// LRUCacheCounter LRU 缓存操作计数（get-hit/get-miss/set/evict）
	LRUCacheCounter *prometheus.CounterVec
)

// Init 注册所有 Prometheus 指标（幂等）
func Init() {
	once.Do(func() {
		EndpointCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "carina_endpoint_requests_total",
				Help: "Total number of requests by resource and method",
			},
			[]string{"resource", "method"},
		)
		EndpointDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "carina_endpoint_duration_seconds",
				Help:    "Request duration by resource and method",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"resource", "method"},
		)
		EndpointCallByService = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "carina_endpoint_call_by_service_total",
				Help: "Total calls by source service",
			},
			[]string{"resource", "method", "source_service"},
		)
		RemoteEndpointCallCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "carina_remote_endpoint_call_total",
				Help: "Total remote endpoint calls",
			},
			[]string{"local_method", "local_resource", "method", "service", "remote_resource"},
		)
		PanicCounter = promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "carina_panic_total",
				Help: "Total number of panics",
			},
		)
		BusinessErrorCounter = promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "carina_business_error_total",
				Help: "Total number of business errors",
			},
		)
		ResourceRetryCounter = promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "carina_resource_retry_total",
				Help: "Total number of resource call retries",
			},
		)
		RestwsGauge = promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "carina_restws_connections",
				Help: "Current WebSocket connections",
			},
		)
		RestwsErrorCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "carina_restws_error_total",
				Help: "Total WebSocket errors by stage",
			},
			[]string{"stage"},
		)
		LRUCacheCounter = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "carina_lru_cache_total",
				Help: "Total LRU cache operations",
			},
			[]string{"cache", "operation"},
		)
	})
}
