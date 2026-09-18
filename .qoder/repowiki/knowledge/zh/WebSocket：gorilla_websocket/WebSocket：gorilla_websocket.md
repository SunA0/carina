---
kind: external_dependency
name: WebSocket：gorilla/websocket
slug: gorilla-websocket
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

ws 包使用 gorilla/websocket（v1.5.3）实现 WebSocket REST Proxy：通过 WS 长连接接收 RestRequest（path/method/params/rid），内部转发到对应 Resource 执行，复用 vanilla 的路由与参数校验机制。