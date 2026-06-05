# Agent 通信流程

Agent A 构造签名消息，附带 capability claim，发送给 server，通过策略校验后，由 server 路由给 Agent B。Agent B 在响应前验证消息 envelope。
