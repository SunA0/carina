---
kind: external_dependency
name: JWT 编解码：golang-jwt
slug: jwt-golang-jwt
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

auth 包使用 golang-jwt/v5（v5.2.1）进行 JWT token 的编解码，Claims 包含 userId/authUserId/tokenType。JWT 认证作为可选中间件（Options.JWTSecret）注入业务上下文，供下游 Resource 获取当前用户身份。