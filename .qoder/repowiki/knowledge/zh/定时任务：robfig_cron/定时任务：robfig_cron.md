---
kind: external_dependency
name: 定时任务：robfig/cron
slug: robfig-cron
category: external_dependency
scope:
    - '**'
---

cron 包基于 robfig/cron/v3（v3.0.1）封装任务运行时，提供 Task 基类、TaskContext（含事务、tracing、metrics）、重试策略与 Pipe 模式（channel-based 生产者/消费者）。与 REST 共享同一套 recovery/事务体系。