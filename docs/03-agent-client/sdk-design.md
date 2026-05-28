# SDK Design

The agent client exposes `ask`, `getMemory`, and `requestAgent`. The local implementation composes memory, LLM completion, and later remote routing behind the same interface.
