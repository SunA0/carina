---
kind: external_dependency
name: 指标采集：Prometheus client_golang
slug: prometheus
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

metrics 包基于 prometheus/client_golang（v1.20.4）定义并注册服务级指标（如 endpoint 计数、source service 计数），通过 middleware.Metrics 中间件在请求路径上埋点。Init 幂等注册，避免重复收集器冲突。