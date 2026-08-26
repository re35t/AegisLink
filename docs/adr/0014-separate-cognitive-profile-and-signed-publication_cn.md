# ADR 0014：分离认知 Profile 与签名发布

## 状态

已接受，取代 ADR 0002。

## 背景

AgentProfile 表达私有认知，AgentFacts 与 AgentCard 分别服务不同的外部信任和通信协议。把三者当成一个公开身份对象，会泄露可能有误的短期观察、混淆信任与通信，也会假设一个实际不存在的 A2A endpoint。

## 决策

- 分离 Impression 与 Confirmed Fact；只有 Owner 可以把 Fact Candidate 提升为 Fact。
- AgentProfile 保持私有，只向 Runtime 注入有上限且明确标注的 Profile context。
- Disclosure Policy 只作用于 identity、capability、Confirmed Fact 与 endpoint；Impression 永不对外披露。
- 使用每 Agent Ed25519 密钥签署版本化 AgentFacts；私钥由运维主密钥加密，并支持域名验证、过期、撤销与隐私友好的 POST 查询。
- 使用官方 A2A Go v2.4.0 类型生成 Owner-only AgentCard Draft；在存在真实 supported interface 前不发布。
- 不实现中央 Index，也不伪造第三方 attestation。

## 后果

系统建立了从私有认知到可信外部发布的完整边界，同时不把自签名冒充为通用 Agent 身份。运维必须安全管理 `AGENT_KEY_ENCRYPTION_KEY`；丢失或更换它会使已有私钥不可用，需要轮换并重新发布。
