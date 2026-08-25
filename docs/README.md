# Documentation status

## Frontend engineering

- [`frontend/PRODUCT.md`](frontend/PRODUCT.md): Personal Agent Console object model, workflows, and product principles.
- [`frontend/DESIGN.md`](frontend/DESIGN.md): concrete layout, token, hierarchy, responsive, interaction-state, and visual-QA rules.
- [`frontend/ARCHITECTURE.md`](frontend/ARCHITECTURE.md): current React/Vite boundaries and incremental feature organization.
- [`frontend/COMPONENTS.md`](frontend/COMPONENTS.md): current component inventory and reuse policy.
- [`frontend/INTERACTIONS.md`](frontend/INTERACTIONS.md): Run, tool, approval, streaming, recovery, empty, and error conventions.
- [`frontend/references/lobehub.md`](frontend/references/lobehub.md): read-only product/design engineering reference and AegisLink adaptations.

The repository-owned Codex workflow is [`../skills/frontend-engineering/SKILL.md`](../skills/frontend-engineering/SKILL.md). Root `AGENTS.md` requires it for substantial frontend changes.

The current implementation baseline is ADR 0004, ADR 0005, ADR 0006, ADR 0013, and the root README. [ADR 0013](./adr/0013-use-gorm-for-postgres-persistence.md) records GORM as the runtime PostgreSQL persistence boundary while Goose remains authoritative for schema migrations.

Except for the current phase plan linked below, documents under `00-overview` through `10-ops` were written for the original TypeScript prototype. They remain as product and security research, but capability routing, Ed25519 identity, RAG, Chroma, WebSocket, gRPC, integrations, and production deployment described there are not implemented in the current Go release unless the root README says otherwise.

The current product direction is the Chinese [Personal Agent OS phase plan](./01-product/personal-agent-os-plan_cn.md). The accepted target stack is Go/Gin, an in-process Eino runtime, PostgreSQL, AG-UI, and React/Vite with `assistant-ui`. The next releases focus on Web, Memory, Skills, and MCP for one personal agent. Network, Group, A2A, and multi-agent orchestration remain out of scope. [ADR 0005](./adr/0005-use-ag-ui-and-assistant-ui.md) records the implemented first AG-UI and `assistant-ui` slice; tool events, approvals, attachments, and native protocol resumption remain incremental work.

[ADR 0006](./adr/0006-use-model-provider-registry-and-react-runtime.md) records the implemented model-provider registry and base Eino ReAct loop. DeepSeek is the current active provider; multiple selectable model profiles remain future work.
