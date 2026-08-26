# 当前安全边界

## 认证与所有权

- 邮箱密码注册使用 Argon2id。浏览器只接收不透明的 `HttpOnly`、`SameSite=Strict` Session Cookie；PostgreSQL 只保存 Token Hash。
- 登录 Human Principal 由服务端 Session 得到，客户端不能提交或覆盖 `principal_id`。
- 资源访问同时校验 Principal Ownership 与权威 Agent Scope。若暴露对象存在性会造成泄漏，非 Owner 查询统一使用 Not Found 语义。
- CORS 与 Origin Check 只允许配置的 Web Origin。生产部署必须启用 Secure Cookie 并正确终止 HTTPS。

## Agent 执行

- AG-UI 输入中的历史消息不是权威历史；服务端会从 PostgreSQL 重新加载 Conversation 与 Agent。
- Composer Selection 只包含不透明 Mention ID 与 Action。Run 启动前，服务端重新解析 Skill/MCP Binding、Tool Risk、连接状态和 Ownership。
- 只有显式启用的 `read-only` MCP Tool 能进入 Runtime。因为尚未实现持久化逐次 Approval，`external-write` 与 `destructive` Tool 始终被阻止。
- Provider Key、MCP Credential、Owner ID 和私有 Chain-of-thought 不得进入 API 响应、Profile Projection、Event 或日志。System Prompt 只允许通过认证且限定 Owner Scope 的 Agent Instructions Endpoint 返回；不得进入 Agent List、Profile Projection、AgentFacts、Event 或日志。

## Profile 披露

Disclosure Policy 在生成 AgentFacts 时强制执行。默认 Private；Restricted 要求 Token Audience 精确匹配，Indexing 只允许 Public AgentFacts Subject。Impression 与 User/Project/Task Fact 永不对外披露。降低披露范围、撤销 Fact、关闭发布与轮换密钥会在事务中撤销当前 Publication。

Message、Tool Result 与 Memory 对 Curator 都是不可信证据。Curator 没有 Tool，只能输出通过 Schema 校验的 Draft，也不能写 Confirmed Fact。Harness 把 Impression 标记为可能有误的低优先级 Context，不注入原始证据或披露元数据。

AgentFacts 使用 Ed25519；Private Key 由 32-byte Base64 `AGENT_KEY_ENCRYPTION_KEY` 通过 AES-256-GCM 加密，并绑定 Agent/Key ID。未配置主密钥时内部 Profile/Impression 正常，但发布与密钥操作被阻止。Query Token Secret 只显示一次，数据库只保存 SHA-256 Hash。

## 存储与运维

- Goose migration 不可变且是 Schema 权威来源；禁止 GORM `AutoMigrate`。
- Repository 集成测试具有破坏性，只接受数据库名以 `_test` 结尾的地址。Compose 测试数据库与开发数据物理分离。
- Skill Import 拒绝路径穿越、绝对路径、Symlink、重复规范化路径、超限 Archive/File 和未声明 Root。保存的 Script 没有执行权限。

## 已知缺口

邮箱验证、密码重置、MFA、登录限流、委托访问、MCP OAuth/Secret Storage、写入/破坏性 Tool Approval、第三方 Credential/Attestation、中央 Discovery Index、A2A Request Authentication、审计保留策略和沙箱 Skill 执行均未实现，部署文档不得声称已具备这些保护。
