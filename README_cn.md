# AegisLink

AegisLink 是一个面向隐私保护的个人 agent 通信平台。每个用户拥有一个个人 agent，具备本地记忆、RAG 检索和 Ed25519 加密身份。服务端负责 agent 注册、capability 权限策略、签名消息路由和审计日志，让多个 agent 能以最小权限方式协作。

当前仓库是早期 TypeScript monorepo 实现，已经可以用于本地开发和后端流程验证，但还不是生产可用版本。

## 当前状态

已实现：

- TypeScript pnpm monorepo 骨架。
- 本地 `agent-client` SDK，支持 `ask`、`getMemory`，远程 agent 请求仍是占位流程。
- 本地 `soul.md` 记忆与 JSONL 对话持久化。
- RAG 分块、hash embedding 和兼容 Chroma 的向量适配器。
- Ed25519 身份生成、规范 JSON 签名和签名验证。
- 本地 agent 问答 CLI demo。
- 内存型后端 MVP：
  - agent 注册与查询；
  - agent 状态更新；
  - capability 签发、列表和撤销；
  - 签名消息 envelope 校验；
  - 基于 capability 的消息路由；
  - allow/deny 审计日志；
  - 用于调试的 JSON snapshot API。
- 基础浏览器控制台，可注册 agent、签发 capability、查看服务端状态。
- 架构、存储、服务端、安全、API、数据模型、集成和运维文档骨架。

暂未实现：

- PostgreSQL 权威元数据持久化。
- 真实组织树和基于角色的策略评估。
- WebSocket/gRPC 流式通信。
- integration gateway 和 IM 适配器。
- 生产级认证、限流、部署 manifests 和可观测性流水线。
- 完整美术和产品级前端 UI。

## 快速启动

安装依赖：

```bash
pnpm install
```

运行类型检查和测试：

```bash
pnpm check
pnpm test
```

启动后端服务和基础 Web 控制台：

```bash
pnpm run dev:server
```

然后打开：

```text
http://127.0.0.1:4321
```

这里要使用 `pnpm run dev:server`，不要用 `pnpm server`：`server` 也是 pnpm 自带命令，`pnpm server` 可能不会执行本仓库里的脚本。

如果要换端口：

```bash
PORT=4322 pnpm run dev:server
```

健康检查：

```bash
curl -sS http://127.0.0.1:4321/healthz
```

查看内存中的服务端状态：

```bash
curl -sS http://127.0.0.1:4321/api/snapshot
```

运行本地 CLI demo：

```bash
pnpm cli ask "What does my agent remember?"
```

## 后端 API

当前后端刻意使用内存状态，先验证核心设计，再接 PostgreSQL。

当前 REST endpoints：

- `GET /healthz`
- `GET /api/snapshot`
- `GET /api/agents`
- `POST /api/agents`
- `PATCH /api/agents/:agentId/status`
- `GET /api/capabilities`
- `POST /api/capabilities`
- `POST /api/capabilities/:capabilityId/revoke`
- `GET /api/messages`
- `POST /api/messages/route`
- `GET /api/audit`

## 项目进展

Phase 1 已完成本地 agent scaffold：本地记忆、CLI、RAG 基础能力、向量适配器、协议类型和加密身份都已实现并有测试覆盖。

Phase 2 已开始。当前后端 MVP 已覆盖 docs 中的主要服务端概念：registry、permission capability 检查、签名 envelope 验证、routing 和 audit。前端当前只做基础功能，用来跑通后端流程，不追求美术设计。

后端下一步优先级：

1. 使用 `packages/database/schema.sql` 把服务端状态从内存迁移到 PostgreSQL。
2. 增加组织成员关系和基于角色的策略评估。
3. 给 client SDK 增加请求签名 helpers，让浏览器/CLI 能端到端构造合法签名 envelope。
4. 增加 WebSocket 消息投递。
5. 扩展服务端测试，覆盖过期、撤销、禁用 agent、重复 nonce 和审计查询。

## 目录

```text
apps/agent-cli          本地 CLI demo
apps/server             后端 MVP 和基础 Web 控制台
packages/agent-client   本地 agent SDK
packages/storage-core   存储接口
packages/storage-local  本地 soul.md 和 JSONL stores
packages/rag-core       分块、embedding、检索
packages/vector-chroma  兼容 Chroma 的向量适配器
packages/crypto-identity Ed25519 身份和签名 helpers
packages/permission-core Capability evaluator
packages/protocol       Message、event 和 capability 类型
docs                    设计与运维文档
data/agents/local-agent 本地 demo agent 数据
```
