---
kind: external_dependency
name: 缓存客户端：go-redis
slug: redis-go-redis
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

cache 包封装 go-redis（v9.6.1），提供全局连接池、常用 KV 操作与前缀清理能力。配置项包含 Addr/Password/DB，初始化后供业务直接调用。