package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var globalTracer trace.Tracer

// Options 追踪配置
type Options struct {
	Enabled    bool
	Endpoint   string  // OTLP HTTP endpoint, 如 "localhost:4318"
	SampleRate float64 // 0.0 ~ 1.0
}

// Init 初始化 OpenTelemetry tracer
func Init(serviceName string, opts Options) (shutdown func(context.Context) error, err error) {
	if !opts.Enabled {
		globalTracer = otel.Tracer(serviceName)
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(opts.Endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create exporter: %w", err)
	}

	if opts.SampleRate <= 0 {
		opts.SampleRate = 0.1
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(opts.SampleRate)),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	globalTracer = tp.Tracer(serviceName)
	return tp.Shutdown, nil
}

// Tracer 返回全局 tracer
func Tracer() trace.Tracer {
	if globalTracer == nil {
		return otel.Tracer("carina")
	}
	return globalTracer
}

// StartSpan 从 context 启动新 span
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name)
}
