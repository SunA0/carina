---
kind: external_dependency
name: 配置加载：Viper
slug: viper
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

config 包使用 Viper（v1.19.0）实现 Base + Load 模式的配置加载：框架提供通用段（Server/DB/Redis/Auth），业务项目通过内嵌 config.Base 扩展本地字段。配置文件目录由下游指定。