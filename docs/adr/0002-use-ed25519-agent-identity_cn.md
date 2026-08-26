# ADR 0002：Ed25519 Agent Identity

## 状态

已被 ADR 0014 取代。

## 背景

TypeScript 原型曾计划用 Ed25519 Key 表达跨 Runtime Agent Identity，并为 Agent-to-Agent 消息签名。当前产品没有公开 Agent Identity、注册网络、跨 Agent Routing、Request Signing Contract 或 Key Lifecycle。

## 决策

当前 Personal Agent OS 不创建 Agent Keypair，也不暴露 Signing API。只有出现明确的外部 Agent 通信工作流后，才重新评估算法、Key Storage、Rotation、Revocation、Recovery 与协议。

数据库 ID、Account Session 和 Disclosure Policy 都不等于加密 Agent Identity。
