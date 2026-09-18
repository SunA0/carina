---
kind: external_dependency
name: 链路追踪：OpenTelemetry（OTLP HTTP）
slug: opentelemetry
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

tracing 包使用 OpenTelemetry（otel v1.30.0）并通过 otlptracehttp 以 OTLP HTTP 方式导出 span。替代了原 tapster vanilla 中的 Jaeger/OpenTracing 方案，成为 Carina 的统一 tracing 后端。