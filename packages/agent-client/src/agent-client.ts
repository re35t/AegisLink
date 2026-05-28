import { randomUUID } from "node:crypto";
import type { MemoryItem, MemoryStore } from "@aegislink/storage-core";
import type { AgentClient, AgentRequestInput, AgentRequestResult, AskInput, AskResult, LlmClient } from "./types.js";

export class LocalAgentClient implements AgentClient {
  constructor(
    private readonly deps: {
      agentId: string;
      memory: MemoryStore;
      llm: LlmClient;
    },
  ) {}

  async ask(input: AskInput): Promise<AskResult> {
    const conversationId = input.conversationId ?? "local";
    const memory =
      input.useMemory === false
        ? { items: [] }
        : await this.deps.memory.search({
            agentId: this.deps.agentId,
            query: input.question,
            limit: 5,
            visibility: ["private", "owner", "team", "org", "public"],
          });

    const prompt = [
      "You are the user's personal AegisLink agent.",
      "Use relevant memory when it is useful, but do not reveal private implementation details.",
      "Relevant memory:",
      JSON.stringify(memory.items, null, 2),
      "User question:",
      input.question,
    ].join("\n\n");

    const answer = await this.deps.llm.complete({ prompt });

    await this.appendConversation(input.question, "user", conversationId);
    await this.appendConversation(answer, "assistant", conversationId);

    return {
      answer,
      conversationId,
      sources: memory.items.map((item) => ({
        id: item.id,
        sourceType: item.sourceType,
        content: item.content,
        score: item.score,
        metadata: item.metadata,
      })),
    };
  }

  async getMemory(input: Parameters<AgentClient["getMemory"]>[0]) {
    return this.deps.memory.search(input);
  }

  async requestAgent(input: AgentRequestInput): Promise<AgentRequestResult> {
    return {
      targetAgentId: input.targetAgentId,
      question: input.question,
      status: "stubbed_until_phase_4",
    };
  }

  private async appendConversation(content: string, role: "user" | "assistant", conversationId: string): Promise<void> {
    const item: MemoryItem = {
      id: randomUUID(),
      agentId: this.deps.agentId,
      content,
      sourceType: "conversation",
      visibility: "private",
      metadata: { role, conversationId },
      createdAt: new Date().toISOString(),
    };
    await this.deps.memory.append(item);
  }
}
