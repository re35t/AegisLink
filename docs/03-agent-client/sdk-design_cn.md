# SDK 设计

agent client 暴露 `ask`、`getMemory` 和 `requestAgent`。本地实现会在同一个接口后组合 memory、LLM completion，以及后续的远程路由能力。
