# ADR 0006：采用 Model Provider Registry 与 Eino ReAct Runtime

状态：已接受并实现。

日期：2026-08-22。

## 背景

ADR 0004 确定使用进程内 Eino Runtime，但首版模型创建依赖硬编码 `switch`，`ChatModelAgent.MaxIterations` 为 1，也没有任何工具，因此只能完成单轮模型生成，不能形成基础 ReAct 循环。AegisLink 当前使用 DeepSeek API，未来需要增加其他模型 Provider 和多个模型配置，但 Conversation 业务层不能依赖具体模型 SDK。

## 决策

- 在 `internal/runtime` 中引入 `ModelProvider` 与并发安全的 `ModelRegistry`。
- Provider 根据稳定 driver 名创建 Eino `ToolCallingChatModel`；当前注册 `deepseek` 和 `openai-compatible`。
- 配置校验只要求非空 driver，具体 driver 是否受支持由 Registry 在应用启动时判断；新增 Provider 不需要修改 Conversation 层。
- 当前模型配置增加稳定的 `MODEL_ID`，为后续多个模型 Profile 和按 Agent 选择模型保留标识。
- DeepSeek 继续作为默认 Provider，当前默认模型名为 `deepseek-v4-flash`；API key 只从 `MODEL_API_KEY` 读取。
- Eino `ChatModelAgent` 使用可配置的 `AGENT_MAX_ITERATIONS`，默认 8，执行模型 → 工具 → 模型的 ReAct 循环。
- 第一版内置只读 `get_current_time` 工具，使用 IANA timezone 并校验输入，用于验证真实工具循环。
- 系统 Instruction 明确要求不能虚构工具结果，也不能向用户暴露私有 chain-of-thought。

## 边界

```text
internal/conversation.Runtime
             │
             ▼
internal/runtime.Eino
  ├── ModelRegistry
  │   ├── deepseek Provider
  │   └── openai-compatible Provider
  └── Eino ChatModelAgent
      └── built-in read-only tools
```

- Eino、DeepSeek 和 OpenAI-compatible SDK 类型不能离开 `internal/runtime`。
- Model Registry 解决 Provider 扩展，不等于已经实现运行时切换多个模型。
- 当前一个进程只激活一个 Model Profile；未来增加选择时，需要将 model ID 持久化到 Agent 或 Run。
- MCP、用户安装工具、Approval 和工具事件的 AG-UI 映射不属于本 ADR。

## 结果

- 基础 Agent 已能执行多轮 ReAct 工具调用，而不再是单轮 Chat Completion 包装。
- DeepSeek 和 OpenAI-compatible 共用同一 Conversation Runtime 接口。
- `/api/v1/bootstrap` 返回稳定 model ID、driver、name 和 `streaming`/`tool-calling` capabilities。
- 自动化测试覆盖 Provider 注册、未知 driver、工具输入校验、ReAct 工具结果回传、流式输出和显式真实 DeepSeek smoke。
- 未来接入新的 Provider 时，在 `internal/runtime` 注册实现并增加配置即可；模型选择与故障切换另行设计。

## 参考

- [DeepSeek Tool Calls](https://api-docs.deepseek.com/guides/tool_calls)
- [DeepSeek API Quick Start](https://api-docs.deepseek.com/quick_start/pricing-details-usd/)
