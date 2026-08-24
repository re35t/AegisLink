# Personal Agent OS 阶段实施规划

状态：当前阶段的产品与架构基线

更新时间：2026-08-23

## 1. 阶段结论

AegisLink 当前按 **Personal Agent OS** 开发，而不是 Multi-Agent 平台。

本阶段只围绕一个长期存在、可观察、可控制的个人 Agent，完成四项核心能力：

1. Web：持续对话与 Agent 管理入口。
2. Memory：可审查、可编辑、可遗忘的长期记忆。
3. Skills：兼容 `SKILL.md` 目录格式的可复用工作流。
4. MCP：由服务端托管连接、凭据、工具目录和权限的 MCP Manager。

Tracing、Approval 和 Permission 不是独立卖点，但属于上述四项能力的基础设施，必须从第一版开始建设。

本阶段明确不实现 Network、Group、Agent Directory、A2A、跨 Agent 路由或复杂 Multi-Agent 编排。接口保留未来扩展点，但不能为了未来能力提前引入分布式系统复杂度。

## 2. 当前基线与规划边界

以下是当前仓库已经具备的实现基线：

- Go、Gin、Eino 组成的模块化单体后端。
- React 19、Vite、TanStack Router 和 TanStack Query Web 客户端。
- OpenAPI 先行的 REST API 与前端类型生成。
- PostgreSQL 持久化 human principals、login accounts、sessions、agents、conversations、messages、runs 和可回放 run events。
- 邮箱密码注册/登录、服务端 Cookie Session，以及 Principal 对 Agent 与聊天资源的所有权隔离。
- SSE 流式响应、断线重连、Run 取消和中断 Run 恢复。
- 可扩展 Model Provider Registry；当前启用 DeepSeek，并保留 OpenAI-compatible Provider。
- Eino 基础 ReAct 循环、可配置最大迭代次数和只读 `get_current_time` 内置工具。
- `POST /api/v1/ag-ui` 文本执行流，以及内部 Run Event 到 AG-UI lifecycle/text event 的 adapter。
- 基于 `assistant-ui` primitives 和 `@assistant-ui/react-ag-ui` 的 Chat Thread、Message、Composer、取消与自动滚动。

当前阶段已经确认的目标技术栈：

| 层级                 | 已确认选型      | 定位                                                                           |
| -------------------- | --------------- | ------------------------------------------------------------------------------ |
| Backend              | Go + Gin        | HTTP API、认证、参数校验和流式传输入口                                         |
| Agent Runtime        | Eino ReAct      | 进程内模型/工具循环与可扩展 Model Provider 适配                                |
| Database             | PostgreSQL      | Conversation、Message、Run、Event 及后续 Memory/Skill/MCP 元数据的权威数据源   |
| Agent ↔ Web Protocol | AG-UI           | Run 生命周期、消息流、工具调用、状态更新和人工审批协议                         |
| Frontend             | React 19 + Vite | Web 应用与构建工具链                                                           |
| Agent UI Library     | `assistant-ui`  | Chat Thread、消息部件、Tool UI、Approval UI、附件及 MCP 管理界面的优先组件来源 |

上述技术栈已经进入实现。当前 AG-UI/assistant-ui 第一切片覆盖文本 Run、流式消息、终态和取消；Tool、Approval、State、附件与原生协议续流仍按小切片补齐。

以下能力已完成 V0 基础切片：

- 按 Principal + Agent 隔离的 Memory CRUD、确认、遗忘和运行时上下文注入。
- 兼容 `SKILL.md` frontmatter 的安装、启停、hash 审计和按需完整内容加载。
- 服务端 MCP Streamable HTTP Client、Server/Tool 管理、发现缓存和只读工具动态调用。
- Chat、Memory、Skills、MCP 的统一产品导航和管理页面。

以下仍是目标能力，不代表已经实现：

- 自动 Memory 候选提取、向量检索、命中原因和 Trace。
- Skill 目录 bundle、scripts/references/assets、安全沙箱和升级来源。
- MCP OAuth/secret reference、资源/Prompt、并发配额、写操作审批和完整回放。
- 完整的工具权限、审批和 Run Trace 页面。
- AG-UI Tool、Approval、State、附件和原生协议续流。
- 基于 `assistant-ui` 的 Tool、Approval 和 MCP 界面。

附加约束：

- PostgreSQL 继续作为权威数据源，不为这一阶段改回 SQLite。
- 保留现有 React/Vite 工程，不迁移到 Next.js；Agent 交互组件优先使用 `assistant-ui`，只为 AegisLink 特有页面补充自定义组件。
- 公共 HTTP 请求或响应变更必须先修改 `contracts/http/v1/openapi.yaml`，再运行 `make generate`。
- 新表和字段只能通过新的 Goose migration 添加，不能重写已经可能执行过的 migration。

## 3. 目标形态

```text
┌────────────────────────────── Web ──────────────────────────────┐
│ Chat │ Memory │ Skills │ MCP │ Agent Profile │ Runs / Traces  │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                         AG-UI over HTTP
                     （保留 SSE 流与事件回放）
                                │
                      ┌─────────▼─────────┐
                      │ Personal Agent    │
                      │ Runtime / Service │
                      └─────────┬─────────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          ▼                     ▼                     ▼
       Memory                 Skills                Tools
   MemoryProvider         SkillProvider         ToolProvider
          │                     │                     │
          │               Skill Registry          MCPManager
          │                                           │
          └──────────────── PostgreSQL ───────────────┤
                                                      ▼
                                                  MCP Servers
```

浏览器只访问 AegisLink HTTP API。MCP Client、OAuth token、bearer token 和其他服务凭据留在服务端，不能返回给浏览器，也不能写入日志或 Run Event。

### 3.1 User Account、Human Principal 与 Personal Agent

当前阶段采用以下用户与 Agent 关联模型：

```text
User Account（登录 / 产品账户）
        │ authenticates
        ▼
Human Principal（现实中的人，也是授权主体）
        │ owns / operates
        │ delegates（预留，V0 不实现共享委托）
        ▼
Personal Agent（长期存在的产品资源）
        ├── Agent Identity
        ├── Agent Key
        ├── Agent Profile
        ├── Memory
        ├── Skills
        └── Capabilities ──► MCP / Tools
        │
        │ represented by
        ▼
Agent Runtime(s)（可重建的执行实例）
```

这些对象不能合并成同一个“用户”概念：

| 对象            | 职责                                                                                       | 不负责                                                     |
| --------------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------- |
| User Account    | 登录标识、认证凭据、账户状态和会话入口；回答“用哪个账户登录”                               | 不直接拥有 Memory、Skill 或 Run，也不能充当 Agent Identity |
| Human Principal | 代表现实中的人，是所有权、授权和审计的主体；回答“谁在控制资源”                             | 不保存密码、登录 token 或 Agent 私钥                       |
| Personal Agent  | Principal 拥有和操作的长期领域资源，聚合 Identity、Profile、Memory、Skills 与 Capabilities | 不等于某次进程、HTTP 请求、Conversation 或 Run             |
| Agent Identity  | Agent 跨 Runtime 保持稳定的身份标识及可公开的身份元数据                                    | 不用于人类登录                                             |
| Agent Key       | Agent 身份签名、轮换和吊销所需的密钥材料或安全引用                                         | 不等于用户密码、Model Provider API key 或 MCP 凭据         |
| Agent Profile   | 用户可编辑的名称、头像、Instructions、模型偏好等展示和行为配置                             | 不保存任何密钥或第三方服务凭据                             |
| Agent Runtime   | 装载 Agent 配置并执行模型、Memory、Skill 与 Tool 调用的运行实例                            | 不拥有 Agent，也不成为持久身份或权威数据源                 |

V0 关系与基数固定为：

- 一个 User Account 只认证到一个 Human Principal；V0 中一个 Principal 也只有一个 Account，但模型上保持二者分离，为以后绑定多种登录方式或企业身份保留空间。
- 一个 Human Principal 可以拥有多个 Personal Agent。产品初期可以默认创建一个 Agent，但数据库和服务接口不能写死“一人只能有一个 Agent”。
- 一个 Personal Agent 只有一个 owner Principal。未来的 delegate 通过独立授权关系表达，不能通过覆盖 owner 或共享账户实现；V0 暂不提供共享委托能力。
- 一个 Personal Agent 有一个稳定的 Agent Identity 和一个 Agent Profile；Agent Key 允许因轮换产生多个历史版本，但同一用途只能有一个当前有效版本。
- Memory、Skill 安装/启用关系和 Capability grant 都必须能关联到 `agent_id`。Skill Package 与 immutable Version 可在同一 Principal 内复用，但某个 Agent 选择哪个 Version、是否启用属于该 Agent 自己的状态。
- 一个 Personal Agent 可以随时间或并发需要由多个 Agent Runtime 表示。Runtime 重启、迁移或扩缩容不能改变 Agent Identity、owner、Profile 或持久化 Memory。

认证和授权调用链统一为：

```text
login/session
  -> resolve User Account
  -> resolve Human Principal
  -> authorize principal_id against agent ownership/delegation
  -> start Agent Runtime with principal_id + agent_id context
```

必须保持以下不变量：

1. 登录标识、邮箱或 session ID 不是资源所有权键；持久资源最终以 `principal_id` 表达所有权，以 `agent_id` 表达 Agent 作用域。
2. Agent Key 不能登录产品账户，User Account 凭据也不能冒充 Agent 对外签名。
3. Model Provider key、MCP token 和其他第三方凭据与 Agent Key 分开管理，只保存安全引用或加密材料，不能进入 Agent Profile、普通日志或 Run Event。
4. 禁用 Account、禁用 Agent 和停止 Runtime 是三个不同操作：分别阻止人类登录、阻止该 Agent 接收新 Run、停止某个执行实例。
5. Conversation、Run、Memory、Skill 和 Capability 的访问必须同时经过 Principal 授权与 Agent 作用域校验，不能只凭客户端提交的 `agent_id` 访问。

当前实现已经通过独立 Goose migration 拆分 `human_principals`、`user_accounts` 和 `account_sessions`。`agents.owner_principal_id` 与 `conversations.owner_principal_id` 明确引用 Principal；注册会在同一事务中创建 Account、Principal 和默认 Personal Agent。Session Cookie 只携带随机不透明 Token，数据库只保存 Token 哈希；密码哈希只属于 Account。不得把密码字段加到 `agents`，也不得使用 Agent Key 代替用户登录。

本节边界止于 Agent Runtime。AgentFacts、NANDA Index、互联网发布与发现、Organization、Group 和 Multi-Agent 关系不属于当前账号功能范围。

## 4. 分层与接口原则

### 4.1 代码边界

- `internal/app`：进程内依赖装配、启动和关闭；只负责连接具体实现，不承载业务规则。
- `internal/httpapi`：Gin 路由、参数校验、错误映射、SSE 输出；不承载业务决策。
- `internal/account`：Account、Human Principal、密码校验、Session 与注册/登录用例；不依赖 Gin 或 PostgreSQL 类型。
- `internal/agent`：Personal Agent 领域模型、查询服务和 owner 作用域；Profile、Identity、Key 后续仍在该模块内演进。
- `internal/conversation`：对话、Run、Memory/Skill/Tool 选择、审批和取消等用例编排。
- `internal/runtime`：Eino 和模型 SDK 适配；SDK 类型不能泄漏到其他层。
- `internal/postgres`：实现各业务模块定义的 Repository 接口，只处理 PostgreSQL 持久化、事务和领域对象映射，不决定何时记忆、何时调用工具。
- 新增子系统可分别落在 `internal/memory`、`internal/skills`、`internal/mcp`、`internal/trace`，对外提供小接口并只使用 `context.Context` 与领域类型。

代码按业务模块组织，保持模块化单体。业务模块在消费侧定义小接口，由 `internal/postgres` 和 `internal/runtime` 提供实现，再由 `internal/app` 手工装配；不引入通用依赖注入框架，也不为每个模块复制 `domain/application/infrastructure` 目录层级。

### 4.2 Provider 边界

底层保留以下职责边界，具体方法随用例和 OpenAPI 设计确定：

```go
type AgentRuntime interface{}
type ModelProvider interface{}
type MemoryProvider interface{}
type SkillProvider interface{}
type ToolProvider interface{}
type ConversationStore interface{}
type TraceProvider interface{}
```

这些接口的目的不是提前抽象所有实现，而是避免业务层直接依赖某个模型、Memory 产品或 MCP SDK。禁止形成 `agent.OpenAI()`、`agent.Mem0()` 一类绑定供应商的业务 API。

未来如需增加网络能力，可以在不改变个人 Agent 核心语义的前提下扩展：

```text
ToolProvider
├── MCPProvider
└── RemoteAgentProvider       # 未来，不属于本阶段

MemoryProvider
├── PersonalMemory
└── GroupMemory               # 未来，不属于本阶段
```

## 5. Web

Web 信息架构固定为以下入口：

```text
AegisLink
├── Chat
├── Memory
├── Skills
├── MCP
├── Runs / Traces
└── Settings
    └── Agent Profile
```

### Chat

- 展示消息流、工具调用、工具结果、附件和错误。
- 流中断后可按事件序号重连，不能通过重复请求制造重复消息或重复工具副作用。
- 需要用户确认时，在对话内展示审批卡片；审批前 Run 处于明确的等待状态。
- 支持取消 Run，并正确呈现 cancelled 终态。

### Agent Profile

- 名称、头像、Instructions、模型和可解释的自主级别。
- Model Provider 配置与 Agent Profile 分离，API key 不进入 Profile 返回值。

### Memory

- 按类型和来源展示近期记忆。
- 支持查看、编辑、确认、遗忘和按来源回到原对话。
- 每次回答可查看实际检索了哪些记忆；不能把 Memory 设计成用户不可见的黑盒。

### Skills

- Installed、Available、Enabled、Disabled 四种可理解状态。
- 展示技能来源、版本、内容摘要、权限需求和最近使用情况。
- 安装、升级、启用、禁用和卸载都要有明确结果。

### MCP

- 展示 Server 连接状态、认证状态、健康状态、工具与资源目录。
- 支持预置连接器和自定义 Server，但服务端必须校验 URL、协议、认证方式和访问范围。
- 用户可逐工具查看与修改权限，不能只提供“已连接”这一层状态。

### UI 技术选择

Web 保持 React 19 + Vite，并正式采用 `assistant-ui` 作为 Agent UI 组件库。实现时优先复用它的 Thread、Composer、消息部件、工具调用、附件、Approval、History 和 MCP 配置能力，不重复手写已有的通用 Agent UI plumbing。

接入策略是增量替换现有聊天组件，而不是重写整个 Web：

- TanStack Router 继续负责应用路由。
- TanStack Query 继续负责普通 REST 资源的服务端状态。
- `assistant-ui` runtime 和 primitives 负责 Agent Thread 与交互部件。
- `@assistant-ui/react-ag-ui` 负责消费后端 AG-UI endpoint。
- Memory、Skills、Profile 和 Runs/Traces 等 AegisLink 特有页面继续使用项目自己的组件与领域 API。

若 `assistant-ui` 组件不能满足产品需求，先通过 wrapper、slot 或自定义 message/tool component 扩展；只有确认上游能力无法覆盖时才自建对应组件。禁止为了使用组件库把工程迁移到 Next.js。

CopilotKit 当前不作为前端依赖；本阶段的目标是独立 Personal Agent 产品，而不是把 Agent 深度嵌入另一个业务应用的页面状态。

## 6. Runtime 与事件模型

### 6.1 Session 不等于 Memory

- Session/Conversation 保存可重放的消息和当前 Run 上下文。
- Working Memory 保存当前任务状态和 scratch 信息，生命周期受 Run 或 Conversation 控制。
- Long-term Memory 保存经过提取、确认或策略筛选的经历与稳定事实。

不得把完整聊天历史直接等同于长期记忆，也不得在没有来源和删除能力的情况下把所有消息永久抽取为记忆。

### 6.2 Agent loop

一个 Run 至少可观察为：

```text
User Message
  → Load conversation/session
  → Retrieve memories
  → Resolve enabled skills
  → Resolve permitted tools
  → Model generation
  → Optional approval
  → Optional tool call and result
  → Final model generation
  → Persist message, events and trace
```

编排属于 `internal/conversation`。模型循环的 SDK 细节留在 `internal/runtime`。

### 6.3 Event Model

现有事件 `run.started`、`message.delta`、`message.completed`、`run.failed` 和 `run.cancelled` 继续作为迁移起点。后续按实际能力增量加入：

```text
run.started
message.started
message.delta
message.completed
tool.call.started
tool.call.completed
tool.call.failed
state.delta
approval.required
approval.resolved
run.succeeded
run.failed
run.cancelled
```

事件 envelope 至少保留 `runId`、单调递增的 `sequence`、`type`、`payload` 和 `occurredAt`，并能关联 `conversationId` 与 `traceId`。所有事件必须可持久化、可排序、可回放、可幂等消费。

AG-UI 已被确定为 Agent 与 Web 之间的协议。对外 endpoint 应遵循 AG-UI 事件和生命周期；内部领域事件不直接依赖前端组件类型，由 HTTP adapter 完成内部 Run Event 与 AG-UI Event 的映射。迁移期间现有事件继续服务当前 Web，但不得再扩展一套与 AG-UI 重叠的私有协议。

OpenAPI 继续描述 Conversation、Memory、Skills、MCP 配置、Permission、Trace 查询和 Run 控制等普通 HTTP API。AG-UI 负责执行态的双向 Agent 交互，两者职责互补。

OpenAI Agents SDK 可作为 Agent、Runner、Session、Tools、MCP、Guardrails、HITL 和 Tracing 等 runtime primitive 的参考实现，但不是 AegisLink 的运行时依赖决策。当前后端继续使用 Go/Eino 和模型中立接口。

## 7. Memory

V0 采用三层语义：

| 层级     | 用途                       | 示例                         | 生命周期         |
| -------- | -------------------------- | ---------------------------- | ---------------- |
| Working  | 当前 Run/Conversation 状态 | 任务步骤、scratch、临时约束  | 短期             |
| Episodic | 发生过的重要事件           | 某次项目决策、一次失败与修复 | 长期但可衰减     |
| Semantic | 相对稳定的事实与偏好       | 用户偏好 Go、项目长期约束    | 长期并可再次确认 |

一个可审查的长期记忆至少包含：

```text
id
owner_id
agent_id
kind
content
confidence
source_uri
created_at
last_confirmed_at
status
```

Memory V0 要求：

- 每条记忆有可追溯来源，优先使用 conversation/message URI。
- 写入策略与检索策略分离；模型提出候选记忆，应用服务决定是否落库。
- 检索结果记录到 Trace，但默认不复制敏感全文到普通日志。
- 用户编辑或遗忘后，后续检索立即生效。
- V0 先用 PostgreSQL；向量检索、时间图谱和外部 Provider 在测量实际需求后再引入。

Letta Memory Blocks、Graphiti temporal knowledge graph 和 Mem0 可作为后续设计或 benchmark 参考，不作为 V0 强依赖。

## 8. Skills

AegisLink 优先兼容以 `SKILL.md` 为入口的目录结构：

```text
my-skill/
├── SKILL.md
├── scripts/
├── references/
└── assets/
```

不另造 `aegis-skill.yaml` 私有格式。Skill Registry 至少提供：

```text
install
upgrade
uninstall
list
search
enable
disable
load_for_task
```

每个已安装技能记录来源、版本、内容 hash、启用范围、所需工具/权限和安装时间。安装时必须防止路径穿越、任意覆盖和未声明脚本执行；技能脚本仍受 Tool Permission 与 Approval 约束，不能因为“已安装”就自动获得任意执行权。

V0 只支持 Personal Skill。Group、Organization 和 Remote Skill 只保留 scope 字段与接口演进空间。

## 9. MCP Manager

目标不是“能连接一个 MCP Server”，而是形成完整的管理子系统：

```text
MCPManager
└── Servers
    ├── connection / transport
    ├── auth reference
    ├── tools
    ├── resources
    ├── permissions
    ├── health / status
    └── last error
```

Runtime 根据当前 Agent、用户、Server 状态和权限动态形成 Available Tools。服务端必须保证：

- 凭据只保存为 secret reference 或加密密文，不进入浏览器、普通日志、Run Event 或模型上下文。
- 自定义 URL 防范 SSRF，并限制重定向、回环地址、私网地址和协议；开发环境例外必须显式配置。
- 工具调用有超时、取消、并发限制、结果大小限制和结构化错误。
- Tool list 缓存按协议能力正确失效，断线或认证失效后不继续暴露旧工具。
- 使用实现时最新的 MCP 规范和兼容 SDK；协议版本作为可观测元数据记录，不依赖旧式教程假定固定 SSE session。

## 10. Permission、Approval 与 Tracing

### 10.1 最小权限模型

| 风险级别         | 默认行为                     | 示例                   |
| ---------------- | ---------------------------- | ---------------------- |
| `read-only`      | 用户授权 Server 后可自动执行 | 读取仓库、查询日历     |
| `external-write` | 每次或按作用域审批           | 创建 Issue、发送消息   |
| `destructive`    | 始终审批，不允许静默持久授权 | 删除文件、删除远端资源 |

权限判断在工具调用前完成，审批结果与实际执行使用同一个 `tool_call_id`，防止批准内容与执行参数不一致。任何“记住本次选择”都必须有清晰 scope、到期时间和撤销入口。

### 10.2 Trace

从第一版统一使用：

```text
conversation_id
run_id
trace_id
span_id
tool_call_id
```

一个 Run Trace 至少能回答：

- 检索了哪些 Memory ID，为什么命中。
- 加载了哪些 Skill 与版本。
- 暴露了哪些 Tool，实际调用了哪些。
- 哪个步骤等待或获得了审批。
- 每次模型调用和工具调用的耗时、状态、token usage 与错误类别。

Trace 必须支持敏感字段脱敏和按保留策略删除。观测性不能成为泄漏 model key、MCP token、用户附件或私密 Memory 的旁路。

## 11. 数据演进

当前 PostgreSQL 权威表为：

```text
human_principals
user_accounts
account_sessions
agents
conversations
messages
runs
run_events
memories
skill_packages
skill_versions
agent_skills
mcp_servers
mcp_tools
```

后续按阶段通过独立 Goose migration 增加，而不是一次性预建所有表：

```text
memory_sources
mcp_credentials        # 只存 secret reference 或密文及其元数据
tool_permissions
approval_requests
trace_spans
```

表名只是当前规划，最终 schema 以对应阶段的领域模型、查询路径、删除语义和威胁分析为准。PostgreSQL 对 conversations、messages、runs、events、memory metadata、skill registry、MCP metadata 和 trace metadata 保持权威性。

## 12. 实施顺序与验收

### Phase 0：基线冻结与契约整理

- 将本文作为当前产品方向，旧 Multi-Agent 文档继续标记为远期研究。
- 记录现有 Run Event 到 AG-UI Event 的映射。
- 为 Provider、Permission 和 Trace 明确包边界，不先实现空泛框架。
- 锁定 Go/Gin、Eino、PostgreSQL、AG-UI、React/Vite 和 `assistant-ui` 技术栈。

验收：现有创建对话、流式回复、重连和取消流程不回归；本文与 README、ADR 0004 不冲突。

### Phase 1：Event、Trace、Permission 基础

- 已完成：在 Go/Gin 后端提供 AG-UI endpoint，并用 adapter 隔离 AG-UI 与 Eino SDK 类型。
- 已完成：在 React/Vite 中接入 `assistant-ui` 与 `@assistant-ui/react-ag-ui`，替换最小 Chat Thread。
- 已完成：Runtime 输出结构化 tool call/result，PostgreSQL 持久化 `tool.started`、`tool.completed`、`tool.failed`，AG-UI 映射为 `TOOL_CALL_*`，并由 assistant-ui Tool UI 与 Run Inspector 展示。
- 下一切片：为 AG-UI run 增加幂等键和基于持久化事件的原生 reconnect/resume，消除迁移期双流。
- 增加 `trace_id` 关联和最小 span persistence。
- 扩展 Approval 生命周期事件，并先用 fake write tool 做自动化测试。
- 实现风险分级和 approval state machine。

验收：`assistant-ui` 能通过 AG-UI 完成创建 Run、文本流、终态、取消与重连；同一个 Run 可完整回放；审批前不会执行工具；取消、重连或重复请求不会重复产生副作用；敏感字段不进入日志和事件。

### Phase 2：Memory V0

- 已完成：Memory domain/repository/service、显式写入、固定预算上下文加载和来源字段。
- 已完成：Memory 列表、编辑、确认和遗忘页面。
- 下一切片：候选提取、向量/相关性检索和消息来源回链。
- 在 Trace 中展示检索 ID 与命中原因。

验收：用户声明的稳定偏好可在新对话中被正确检索；episodic 与 semantic 不混写；删除后不再被检索；来源可回到原消息。

### Phase 3：Skills V0

- 已完成：inline `SKILL.md` frontmatter 校验、安装、启停、内容 hash 与渐进式 `load_skill` 加载。
- 已完成：Principal 级 `skill_packages`、immutable `skill_versions` 与 Agent 级 binding 分离；不同 Agent 可选择同名 Package 的不同 Version。
- 已完成：Skills 管理页面。
- 下一切片：目录 bundle、来源升级、scripts/references/assets 的安全边界和 Skill 使用 Trace。
- 用无外部依赖的示例 Skill 覆盖端到端测试。

验收：无效或越界 bundle 被拒绝；disabled Skill 不进入 Runtime；版本/hash 可审计；脚本不会绕过工具权限。

### Phase 4：MCP V0

- 已完成：基于官方 Go SDK 的服务端 Streamable HTTP Client 与 MCP Manager。
- 已完成：Server、Tool 风险、启停、发现、状态和错误 UI；未受信任的 Server annotation 不会自动授权工具。
- 已完成：只有显式启用且标为 `read-only` 的工具进入单次 Agent Runtime。
- 下一切片：OAuth/secret reference、写/破坏性工具的逐调用 Approval 和可回放 fake Server 验收。

验收：凭据不出服务端；工具目录按 Agent/用户权限过滤；断线、超时、取消、审批和错误均可回放；自动化测试使用 fake MCP Server。

### Phase 5：Web 统一体验

- 统一 Chat、Memory、Skills、MCP、Agent Profile 和 Runs/Traces 导航。
- 使用 `assistant-ui` primitives 统一消息、工具、附件、Approval、History 和 MCP 交互，并为 AegisLink 特有页面提供一致外观。
- 增加附件、工具结果、审批和错误的可访问性与移动端验收。

验收：浏览器覆盖创建对话、流式回复、重连、取消、Memory 管理、Skill 启停、MCP 配置与审批；桌面和移动布局均可完成核心流程。

只有完成上述阶段并出现真实的第二个独立 Agent 需求后，才进入 Network/A2A 设计。

## 13. 学习与技术调研优先级

| 优先级 | 主题                         | 直接产出                                |
| ------ | ---------------------------- | --------------------------------------- |
| P0     | Agent loop / harness         | 可取消、可恢复、可审计的单 Agent Run    |
| P0     | MCP Client + Permission      | 安全、动态的工具能力                    |
| P0     | Agent Skills                 | 可复用且可审计的行为包                  |
| P0     | Memory architecture          | Personal Agent 的长期连续性             |
| P0     | Streaming / Event Model      | 稳定 Web 交互与回放                     |
| P0     | AG-UI + assistant-ui         | 已确认的前后端 Agent 交互协议与组件实现 |
| P1     | Tracing / Observability      | Run 调试与成本定位                      |
| P1     | HITL / Approval              | 外部写入和破坏性操作的控制点            |
| P2     | Graphiti / temporal KG       | Memory 的时间关系实验                   |
| P2     | A2A                          | 等真实第二个 Agent 出现                 |
| P3     | Routing / distributed system | 等 Central Server 阶段                  |

本阶段不投入 LangGraph 复杂图编排、CrewAI、AutoGen 或其他 Multi-Agent orchestration。

## 14. 选型与参考资料

| 项目                    | 状态                        | 资料                                                                                                                                                                                        |
| ----------------------- | --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AG-UI                   | 文本 Run 第一切片已实现     | [AG-UI Overview](https://docs.ag-ui.com/introduction)                                                                                                                                       |
| `assistant-ui`          | Chat/Tool 第一切片已实现    | [文档](https://www.assistant-ui.com/docs)、[AG-UI runtime](https://www.assistant-ui.com/docs/runtimes/ag-ui/overview)、[MCP Config Dialog](https://www.assistant-ui.com/docs/ui/mcp-config) |
| MCP                     | Streamable HTTP V0 已实现   | [Model Context Protocol](https://modelcontextprotocol.io/)、[官方 Go SDK](https://github.com/modelcontextprotocol/go-sdk)                                                                    |
| OpenAI Agents SDK       | 仅作 Runtime 设计参考       | [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/)                                                                                                                         |
| Agent Skills            | inline `SKILL.md` V0 已实现 | [Agent Skills 规范](https://github.com/agentskills/agentskills/blob/main/docs/specification.mdx)                                                                                            |
| Letta / Graphiti / Mem0 | Memory 后续研究或 benchmark | [Letta](https://docs.letta.com/tutorials/attaching-detaching-blocks/)、[Graphiti](https://help.getzep.com/graphiti/getting-started/welcome)、[Mem0](https://docs.mem0.ai/platform/overview) |
