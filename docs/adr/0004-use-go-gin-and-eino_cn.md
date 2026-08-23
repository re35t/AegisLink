# ADR 0004：采用 Go、Gin 和进程内 Eino Runtime

状态：已接受。

## 背景

TypeScript 原型验证了本地记忆、身份与 Capability 概念，但服务端仍是内存实现，也没有形成可用的聊天产品。第一轮重构优先追求代码易读、快速交付和可持久化的 Web 对话链路。

## 决策

- 使用 Go 模块化单体，Gin 只存在于 HTTP Gateway。
- 使用 Eino ADK 的 `ChatModelAgent` 和 `Runner` 作为进程内 Agent Runtime。
- 通过配置支持 DeepSeek 与 OpenAI-compatible 模型入口。
- 使用 PostgreSQL 保存用户、Agent、Conversation、Message、Run 和可回放事件。
- Web 使用 React、Vite、TanStack Router 和 TanStack Query。
- 删除全部后端 TypeScript，旧 Capability、签名、RAG 和跨 Agent 能力延后实现。

## 结果

首版只有一个服务部署单元，暂不维护第二套 Runtime 协议。模型 SDK 类型不能越过 Runtime 接口。PostgreSQL 事件支持 SSE 回放；活动 Eino 执行仍在进程内，服务重启后中断 Run 会明确失败。
