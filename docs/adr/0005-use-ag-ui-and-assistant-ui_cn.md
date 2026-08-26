# ADR 0005：采用 AG-UI 与 assistant-ui

状态：已接受，第一阶段已实现。

日期：2026-08-22；第一阶段实现于 2026-08-23。

## 背景

ADR 0004 已确定 Go/Gin 模块化单体、进程内 Eino Runtime、PostgreSQL 和 React/Vite。当前 Web 已具备基本对话、SSE 流式响应、事件回放、重连和取消，但消息、工具、审批、附件和 MCP UI 尚未形成统一组件体系，Agent 与 Web 之间也只有项目私有的 Run Event 子集。

AegisLink 当前作为 Personal Agent OS 开发，需要优先复用成熟的 Agent UI 组件，同时为 Memory、Skills、MCP、Tool、Approval 和 Trace 建立稳定的前后端协议。

## 决策

- 后端继续使用 Go/Gin，Gin 只存在于 `internal/httpapi`。
- Agent Runtime 继续使用 Eino；Eino 和模型 SDK 类型只存在于 `internal/runtime`。
- PostgreSQL 继续作为 Conversation、Message、Run、Event 及后续 Agent 元数据的权威数据源。
- Agent 与 Web 的执行态协议采用 AG-UI，覆盖 Run 生命周期、文本流、工具调用、状态更新、取消和人工审批。
- 前端继续使用 React 19 + Vite，不迁移到 Next.js。
- 前端采用 `assistant-ui` 组件库，并优先使用 `@assistant-ui/react-ag-ui` 连接 AG-UI endpoint。
- 优先复用 `assistant-ui` 的 Thread、Composer、消息部件、Tool UI、Attachment、Approval、History 和 MCP 管理组件；AegisLink 特有的 Memory、Skills、Profile 和 Runs/Traces 页面保留自有领域组件。
- OpenAPI 继续管理普通 REST 资源和控制接口；AG-UI 管理执行态 Agent 交互，两者不互相替代。

## 边界

```text
React/Vite + assistant-ui
          │
       AG-UI
          │
Go/Gin HTTP adapter
          │
internal/conversation
          │
internal/runtime (Eino)
          │
     Model Provider

PostgreSQL 保存 Conversation、Run、Event 和配置元数据；尚未实现 Trace 存储。
```

- AG-UI wire type 不能进入 Conversation 领域接口，由 HTTP adapter 与内部 Run Event 互相映射。
- Eino SDK type 不能进入 HTTP handler 或 Web contract。
- MCP credential 只留在服务端，不能经 AG-UI、OpenAPI response、Run Event 或日志返回浏览器。
- `assistant-ui` 不能反向决定后端领域模型；无法直接复用的组件先通过 wrapper、slot 或自定义 message/tool component 扩展。
- 现有 SSE API 在迁移期间保持可用，但不再扩展与 AG-UI 重叠的私有事件协议。

## 结果

- 后端已新增 `POST /api/v1/ag-ui` 和内部 Run Event 到 AG-UI lifecycle/text event 的映射层。
- Web 已安装 `@assistant-ui/react`、`@assistant-ui/react-ag-ui` 与 `@ag-ui/client`，并用 assistant-ui primitives 替换正常 Chat Thread。
- PostgreSQL 历史继续通过 REST 加载；AG-UI 请求中的旧消息不作为可信历史。旧 Run Event SSE 在迁移期继续负责页面刷新后的活动 Run 回放。
- 已实现文本流、正常/失败终态、请求取消，以及由持久化 Run Event 支撑的 AG-UI Tool Call Start/Arguments/Result Event。Approval、附件、State 和原生 AG-UI 重连/恢复仍待增量实现。
- 自动化验收覆盖文本流、工具调用、取消、断线重连和事件回放；只有服务端能够持久化并恢复 Approval 后，审批流程才成为强制验收项。
- CopilotKit、Next.js 和 Multi-Agent/A2A 不属于本次决策。
