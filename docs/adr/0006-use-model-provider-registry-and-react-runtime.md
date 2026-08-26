# ADR 0006: Use a model-provider registry and Eino ReAct runtime

Status: Accepted and implemented; Runtime/Harness ownership revised by [ADR 0015](0015-separate-agent-harness-from-eino-runtime.md).

Date: 2026-08-22.

## Context

ADR 0004 selected an in-process Eino runtime, but the first implementation created models through a hard-coded switch, set `ChatModelAgent.MaxIterations` to 1, and had no tools. It could only perform a single model generation instead of a base ReAct loop. AegisLink currently uses the DeepSeek API and must support additional providers and model profiles later without coupling conversation orchestration to model SDKs.

## Decision

- Add `ModelProvider` and a concurrency-safe `ModelRegistry` inside `internal/runtime`.
- Providers create Eino `ToolCallingChatModel` instances by stable driver name. Register `deepseek` and `openai-compatible` now.
- Generic configuration requires a non-empty driver; the Registry validates actual driver support during application startup. Adding a provider does not change the conversation layer.
- Add a stable `MODEL_ID` for the active model profile, leaving room for multiple profiles and per-Agent selection later.
- Keep DeepSeek as the default provider and use `deepseek-v4-flash` as the current default model name. Read the API key only from `MODEL_API_KEY`.
- Configure Eino `ChatModelAgent` with `AGENT_MAX_ITERATIONS`, defaulting to 8, to run model → tool → model ReAct cycles.
- Include a read-only `get_current_time` tool with validated IANA timezones to prove the real tool loop. ADR 0015 later moved this product Tool into Harness.
- Extend the system instruction so the model cannot invent tool results or expose private chain-of-thought. ADR 0015 later moved product instruction composition into Harness.

## Boundaries

```text
internal/conversation.Harness
             │
             ▼
internal/harness
             │
             ▼
internal/runtime.Eino
  ├── ModelRegistry
  │   ├── deepseek provider
  │   └── openai-compatible provider
  └── Eino ChatModelAgent
```

- Eino, DeepSeek, and OpenAI-compatible SDK types remain inside `internal/runtime`.
- The Registry enables provider extension; it does not yet implement runtime selection among multiple configured models.
- One process currently activates one model profile. Future selection must persist the model ID on the Agent or Run.
- MCP, user-installed tools, Approval, and AG-UI tool-event mapping are outside this ADR.

## Consequences

- The base Agent can execute multi-turn ReAct tool calls instead of wrapping one Chat Completion.
- DeepSeek and OpenAI-compatible providers share one domain-neutral Runtime interface behind the Agent Harness.
- `/api/v1/bootstrap` exposes stable model ID, driver, name, and `streaming`/`tool-calling` capabilities.
- Automated tests cover provider registration, unknown drivers, tool input validation, ReAct tool-result feedback, streaming output, and an explicit real DeepSeek smoke.
- Adding another provider requires a new implementation and registration under `internal/runtime`; model selection and failover remain separate work.

## References

- [DeepSeek Tool Calls](https://api-docs.deepseek.com/guides/tool_calls)
- [DeepSeek API Quick Start](https://api-docs.deepseek.com/quick_start/pricing-details-usd/)
